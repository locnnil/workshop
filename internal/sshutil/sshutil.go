// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package sshutil

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

type PublicKey struct {
	key     ssh.PublicKey
	comment string
}

type PrivateKey struct {
	key     crypto.PrivateKey
	comment string
}

func (k PublicKey) String() string {
	data := ssh.MarshalAuthorizedKey(k.key)
	data = bytes.TrimSuffix(data, []byte("\n"))
	if k.comment != "" {
		data = fmt.Append(data, " ", k.comment)
	}
	return string(data)
}

func (k PrivateKey) MarshalText() ([]byte, error) {
	block, err := ssh.MarshalPrivateKey(k.key, k.comment)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(block), nil
}

func GenerateKey(comment string) (*PublicKey, *PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ssh-keygen -C %q: %w", comment, err)
	}

	wrapped, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}

	return &PublicKey{key: wrapped, comment: comment}, &PrivateKey{key: priv, comment: comment}, nil
}
