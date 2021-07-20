package keystore

import (
	"crypto"
	"encoding/json"

	"github.com/ThalesIgnite/crypto11"
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

var label = []byte("teleport")

// HSMConfigConfig is used to pass HSM client configuration parameters.
type HSMConfig struct {
	// Path is the path to the PKCS11 module.
	Path string
	// SlotNumber is the PKCS11 slot to use.
	SlotNumber *int
	// TokenLabel is the label of the PKCS11 token to use.
	TokenLabel string
	// Pin is the PKCS11 pin for the given token.
	Pin string

	// HostUUID is the UUID of the local auth server this HSM is connected to.
	HostUUID string
}

type hsmKeyStore struct {
	ctx      *crypto11.Context
	hostUUID string
}

func NewHSMKeyStore(config *HSMConfig) (KeyStore, error) {
	cryptoConfig := &crypto11.Config{
		Path:       config.Path,
		TokenLabel: config.TokenLabel,
		SlotNumber: config.SlotNumber,
		Pin:        config.Pin,
	}
	ctx, err := crypto11.Configure(cryptoConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &hsmKeyStore{
		ctx:      ctx,
		hostUUID: config.HostUUID,
	}, nil
}

// GenerateRSA creates a new RSA private key and returns its identifier and a
// crypto.Signer. The returned identifier can be passed to GetSigner later to
// get the same crypto.Signer.
func (c *hsmKeyStore) GenerateRSA() ([]byte, crypto.Signer, error) {
	var id uuid.UUID
	var signer crypto.Signer
	var err error

	// Some HSMs (like YubiHSM2) will truncate the passed ID to as few as 2
	// bytes. There's not a great way to detect this and I don't want to limit
	// the ID to 2 bytes on all systems, so for now we will generate a few
	// random IDs and hope to avoid a collision. Ideally Teleport should be the
	// only thing creating keys for this token and there should only be 10 keys
	// per HSM at a given time:
	// 2(rotation phases) * (4(SSH and TLS for User and Host CA) + 1(JWT CA))
	for iterations := 0; iterations < 32 && (err != nil || signer == nil); iterations++ {
		id, err = uuid.NewRandom()
		if err != nil {
			return nil, nil, err
		}
		signer, err = c.ctx.GenerateRSAKeyPairWithLabel(id[:], label, teleport.RSAKeySize)
	}
	if signer == nil {
		return nil, nil, trace.Wrap(err, "failed to create RSA key in hsm, resources may be exhausted")
	}

	key := keyID{
		HostID: c.hostUUID,
		KeyID:  id.String(),
	}

	keyID, err := key.marshal()
	if err != nil {
		return nil, nil, err
	}
	return keyID, signer, nil
}

// GetSigner returns a crypto.Signer for the given key identifier, if it is found.
func (c *hsmKeyStore) GetSigner(rawKey []byte) (crypto.Signer, error) {
	keyType := KeyType(rawKey)
	switch keyType {
	case types.PrivateKeyType_PKCS11:
		keyID, err := parseKeyID(rawKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if keyID.HostID != c.hostUUID {
			return nil, trace.NotFound("pkcs11 key is for different host")
		}
		pkcs11ID, err := keyID.pkcs11Key()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signer, err := c.ctx.FindKeyPair(pkcs11ID, label)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if signer == nil {
			return nil, trace.NotFound("failed to find keypair for given id")
		}
		return signer, nil
	case types.PrivateKeyType_RAW:
		signer, err := utils.ParsePrivateKeyPEM(rawKey)
		return signer, trace.Wrap(err)
	}
	return nil, trace.BadParameter("unrecognized key type %s", keyType.String())
}

func (c *hsmKeyStore) selectTLSKeyPair(ca types.CertAuthority) (*types.TLSKeyPair, error) {
	keyPairs := ca.GetActiveKeys().TLS
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no TLS key pairs found in CA for %q", ca.GetClusterName())
	}
	// prefer hsm key
	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_PKCS11 {
			keyID, err := parseKeyID(keyPair.Key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if keyID.HostID != c.hostUUID {
				continue
			}
			return keyPair, nil
		}
	}
	// if there are no hsm keys for this host, check for a raw key
	for _, keyPair := range keyPairs {
		if keyPair.KeyType == types.PrivateKeyType_RAW {
			return keyPair, nil
		}
	}
	return nil, trace.NotFound("no TLS key pairs found in CA for %q", ca.GetClusterName())
}

// GetTLSCertAndSigner selects the local TLS keypair and returns the raw TLS cert and crypto.Signer.
func (c *hsmKeyStore) GetTLSCertAndSigner(ca types.CertAuthority) ([]byte, crypto.Signer, error) {
	keyPair, err := c.selectTLSKeyPair(ca)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// if there is no key, this CA may only be used for checking
	if len(keyPair.Key) == 0 {
		return keyPair.Cert, nil, nil
	}

	signer, err := c.GetSigner(keyPair.Key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keyPair.Cert, signer, nil
}

func (c *hsmKeyStore) selectSSHKeyPair(ca types.CertAuthority) (*types.SSHKeyPair, error) {
	keyPairs := ca.GetActiveKeys().SSH
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no SSH key pairs found in CA for %q", ca.GetClusterName())
	}
	// prefer hsm key
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_PKCS11 {
			keyID, err := parseKeyID(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if keyID.HostID != c.hostUUID {
				continue
			}
			return keyPair, nil
		}
	}
	// if there are no hsm keys for this host, check for a raw key
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			return keyPair, nil
		}
	}
	return nil, trace.NotFound("no SSH key pairs found in CA for %q", ca.GetClusterName())
}

// GetSSHSigner selects the local SSH keypair and returns an ssh.Signer.
func (c *hsmKeyStore) GetSSHSigner(ca types.CertAuthority) (sshSigner ssh.Signer, err error) {
	keyPair, err := c.selectSSHKeyPair(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch keyPair.PrivateKeyType {
	case types.PrivateKeyType_RAW:
		sshSigner, err = ssh.ParsePrivateKey(keyPair.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case types.PrivateKeyType_PKCS11:
		signer, err := c.GetSigner(keyPair.PrivateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sshSigner, err = ssh.NewSignerFromSigner(signer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unrecognized key type %q", keyPair.PrivateKeyType.String())
	}

	sshSigner = sshutils.AlgSigner(sshSigner, sshutils.GetSigningAlgName(ca))
	return sshSigner, nil
}

// GetJWTSigner returns the active jwt signer used to sign tokens.
func (c *hsmKeyStore) GetJWTSigner(ca types.CertAuthority) (crypto.Signer, error) {
	keyPairs := ca.GetActiveKeys().JWT
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("no JWT keypairs found")
	}
	// prefer hsm key if there is one
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_PKCS11 {
			keyID, err := parseKeyID(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if keyID.HostID != c.hostUUID {
				continue
			}
			signer, err := c.GetSigner(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	// if there are no hsm keys for this host, check for a raw key
	for _, keyPair := range keyPairs {
		if keyPair.PrivateKeyType == types.PrivateKeyType_RAW {
			signer, err := utils.ParsePrivateKey(keyPair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return signer, nil
		}
	}
	return nil, trace.NotFound("no JWT key pairs found in CA for %q", ca.GetClusterName())
}

// DeleteKey deletes the given key from the HSM
func (c *hsmKeyStore) DeleteKey(rawKey []byte) error {
	keyID, err := parseKeyID(rawKey)
	if err != nil {
		return trace.Wrap(err)
	}
	if keyID.HostID != c.hostUUID {
		return trace.NotFound("pkcs11 key is for different host")
	}
	pkcs11ID, err := keyID.pkcs11Key()
	if err != nil {
		return trace.Wrap(err)
	}
	signer, err := c.ctx.FindKeyPair(pkcs11ID, label)
	if err != nil {
		return trace.Wrap(err)
	}
	if signer == nil {
		return trace.NotFound("failed to find keypair for given id")
	}
	return trace.Wrap(signer.Delete())
}

type keyID struct {
	HostID string `json:"host_id"`
	KeyID  string `json:"key_id"`
}

func (k keyID) marshal() ([]byte, error) {
	buf, err := json.Marshal(k)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf = append(append([]byte{}, pkcs11Prefix...), buf...)
	return buf, nil
}

func (k keyID) pkcs11Key() ([]byte, error) {
	id, err := uuid.Parse(k.KeyID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return id[:], nil
}

func parseKeyID(key []byte) (keyID, error) {
	var keyID keyID
	if KeyType(key) != types.PrivateKeyType_PKCS11 {
		return keyID, trace.BadParameter("unable to parse invalid pkcs11 key")
	}
	// strip pkcs11: prefix
	key = key[len(pkcs11Prefix):]
	if err := json.Unmarshal(key, &keyID); err != nil {
		//return keyID, trace.BadParameter("unable to parse invalid pkcs11 key")
		return keyID, trace.Wrap(err)
	}
	return keyID, nil
}