// Copyright 2019 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/sorintlab/agola/internal/etcd"
	"github.com/sorintlab/agola/internal/objectstorage"
	"github.com/sorintlab/agola/internal/objectstorage/posix"
	"github.com/sorintlab/agola/internal/objectstorage/s3"
	"github.com/sorintlab/agola/internal/services/config"
	"go.uber.org/zap"
)

const (
	StorePrefix = "agola"
)

// WriteFileAtomicFunc atomically writes a file, it achieves this by creating a
// temporary file and then moving it. writeFunc is the func that will write
// data to the file.
// This function is taken from
//   https://github.com/youtube/vitess/blob/master/go/ioutil2/ioutil.go
// Copyright 2012, Google Inc. BSD-license, see licenses/LICENSE-BSD-3-Clause
func WriteFileAtomicFunc(filename string, perm os.FileMode, writeFunc func(f io.Writer) error) error {
	dir, name := path.Split(filename)
	f, err := ioutil.TempFile(dir, name)
	if err != nil {
		return err
	}
	err = writeFunc(f)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if permErr := os.Chmod(f.Name(), perm); err == nil {
		err = permErr
	}
	if err == nil {
		err = os.Rename(f.Name(), filename)
	}
	// Any err should result in full cleanup.
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}

// WriteFileAtomic atomically writes a file
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	return WriteFileAtomicFunc(filename, perm,
		func(f io.Writer) error {
			_, err := f.Write(data)
			return err
		})
}

func NewObjectStorage(c *config.ObjectStorage) (*objectstorage.ObjStorage, error) {
	var (
		err error
		ost objectstorage.Storage
	)

	switch c.Type {
	case config.ObjectStorageTypePosix:
		ost, err = posix.New(c.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create posix object storage")
		}
	case config.ObjectStorageTypeS3:
		// minio golang client doesn't accept an url as an endpoint
		endpoint := c.Endpoint
		secure := !c.DisableTLS
		if u, err := url.Parse(c.Endpoint); err == nil {
			endpoint = u.Host
			switch u.Scheme {
			case "https":
				secure = true
			case "http":
				secure = false
			default:
				return nil, errors.Errorf("wrong s3 endpoint scheme %q (must be http or https)", u.Scheme)
			}
		}
		ost, err = s3.New(c.Bucket, c.Location, endpoint, c.AccessKey, c.SecretAccessKey, secure)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create s3 object storage")
		}
	}

	return objectstorage.NewObjStorage(ost, "/"), nil
}

func NewEtcd(c *config.Etcd, logger *zap.Logger, prefix string) (*etcd.Store, error) {
	e, err := etcd.New(etcd.Config{
		Logger:        logger,
		Endpoints:     c.Endpoints,
		Prefix:        prefix,
		CertFile:      c.TLSCertFile,
		KeyFile:       c.TLSKeyFile,
		CAFile:        c.TLSCAFile,
		SkipTLSVerify: c.TLSSkipVerify,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create etcd store")
	}

	return e, nil
}
