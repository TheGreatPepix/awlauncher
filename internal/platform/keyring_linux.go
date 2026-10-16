package platform

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// Sessions are sealed with a key kept in the Secret Service (KWallet,
// GNOME Keyring). When it is missing or locked, as it can be in Steam's
// Game Mode where nothing can ask for its password, they stay plain files
// that only the user can read.

var sealedMagic = []byte("AWSS\x00\x01")

const keyringTimeout = 3 * time.Second

var keyringAttrs = map[string]string{"application": "awlauncher", "purpose": "session-key"}

var tokenKey = func(create bool) ([]byte, error) { return cachedKey.get(create) }

var cachedKey keyCache

type keyCache struct {
	mu  sync.Mutex
	key []byte
}

func (c *keyCache) get(create bool) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key != nil {
		return c.key, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyringTimeout)
	defer cancel()
	key, err := keyringKey(ctx, create)
	if err != nil {
		return nil, err
	}
	c.key = key
	return key, nil
}

func Protect(data []byte) ([]byte, error) {
	key, err := tokenKey(true)
	if err != nil {
		log.Printf("Sessions are kept unencrypted: %v\n", err)
		return data, nil
	}
	return seal(key, data)
}

func Unprotect(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, sealedMagic) {
		return data, nil
	}
	key, err := tokenKey(false)
	if err != nil {
		log.Printf("Cannot open a saved session: %v\n", err)
		return nil, ErrKeyUnavailable
	}
	return open(key, data)
}

func seal(key, data []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append(bytes.Clone(sealedMagic), nonce...)
	return gcm.Seal(out, nonce, data, sealedMagic), nil
}

func open(key, data []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	body := data[len(sealedMagic):]
	if len(body) < gcm.NonceSize() {
		return nil, errors.New("saved session is damaged")
	}
	clear, err := gcm.Open(nil, body[:gcm.NonceSize()], body[gcm.NonceSize():], sealedMagic)
	if err != nil {
		return nil, ErrKeyUnavailable
	}
	return clear, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

type secret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

const (
	secretsName = "org.freedesktop.secrets"
	secretsPath = dbus.ObjectPath("/org/freedesktop/secrets")
	secretsIfc  = "org.freedesktop.Secret."
)

// keyringKey reads the session key from the Secret Service, or with create
// stores a new one in the default collection. It never asks for a password.
func keyringKey(ctx context.Context, create bool) ([]byte, error) {
	conn, err := dbus.SessionBusPrivate(dbus.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("no session bus: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		return nil, err
	}
	service := conn.Object(secretsName, secretsPath)
	var algorithm dbus.Variant
	var session dbus.ObjectPath
	if err := service.CallWithContext(ctx, secretsIfc+"Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&algorithm, &session); err != nil {
		return nil, fmt.Errorf("no Secret Service: %w", err)
	}
	defer conn.Object(secretsName, session).CallWithContext(ctx, secretsIfc+"Session.Close", 0)
	var unlocked, locked []dbus.ObjectPath
	if err := service.CallWithContext(ctx, secretsIfc+"Service.SearchItems", 0, keyringAttrs).Store(&unlocked, &locked); err != nil {
		return nil, err
	}
	for _, item := range unlocked {
		var s secret
		if err := conn.Object(secretsName, item).CallWithContext(ctx, secretsIfc+"Item.GetSecret", 0, session).Store(&s); err == nil && len(s.Value) == 32 {
			return s.Value, nil
		}
	}
	if len(locked) > 0 || !create {
		return nil, ErrKeyUnavailable
	}
	var collection dbus.ObjectPath
	if err := service.CallWithContext(ctx, secretsIfc+"Service.ReadAlias", 0, "default").Store(&collection); err != nil || collection == "/" {
		return nil, errors.New("the keyring has no default collection")
	}
	coll := conn.Object(secretsName, collection)
	if v, err := coll.GetProperty(secretsIfc + "Collection.Locked"); err != nil || v.Value() != false {
		return nil, ErrKeyUnavailable
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	props := map[string]dbus.Variant{
		secretsIfc + "Item.Label":      dbus.MakeVariant("AWLauncher sessions"),
		secretsIfc + "Item.Attributes": dbus.MakeVariant(keyringAttrs),
	}
	var item, prompt dbus.ObjectPath
	value := secret{Session: session, Value: key, ContentType: "application/octet-stream"}
	if err := coll.CallWithContext(ctx, secretsIfc+"Collection.CreateItem", 0, props, value, true).Store(&item, &prompt); err != nil {
		return nil, err
	}
	if prompt != "/" {
		return nil, ErrKeyUnavailable
	}
	return key, nil
}
