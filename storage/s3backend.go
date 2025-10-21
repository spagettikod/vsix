package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spagettikod/vsix/vscode"
)

type S3Config struct {
	Endpoint        string
	Bucket          string
	CredentialsFile string
	Profile         string
	Prefix          string
	useSSL          bool
	apcDelta        bool
}

func NewS3Config(urlStr, bucket, prefix, credentialsFile, profile string, apcDelta bool) (S3Config, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return S3Config{}, err
	}
	return S3Config{
		Endpoint:        u.Host,
		Bucket:          bucket,
		CredentialsFile: credentialsFile,
		Profile:         profile,
		Prefix:          prefix,
		useSSL:          u.Scheme == "https",
		apcDelta:        apcDelta,
	}, nil
}

type S3Backend struct {
	BaseBackend
	c   *minio.Client
	bkt string
	cfg S3Config
}

func NewS3Backend(cfg S3Config) (Backend, error) {
	c, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewFileAWSCredentials(cfg.CredentialsFile, cfg.Profile),
		Secure: cfg.useSSL,
	})
	if err != nil {
		return nil, err
	}
	s3 := &S3Backend{
		c:   c,
		bkt: cfg.Bucket,
		cfg: cfg,
	}
	s3.BaseBackend = BaseBackend{impl: s3}
	return s3, nil
}

func (s3 S3Backend) ListVersionTags(uid vscode.UniqueID) ([]vscode.VersionTag, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tags := map[string]vscode.VersionTag{}

	var s3err error
	prefix := filepath.Join(s3.cfg.Prefix, ExtensionPath(uid))
	for obj := range s3.c.ListObjects(ctx, s3.bkt, minio.ListObjectsOptions{Prefix: prefix + "/", Recursive: true}) {
		if obj.Err != nil {
			s3err = obj.Err
			slog.Error("list version tag error", "error", s3err, "uid", uid.String())
			continue
		}
		key := obj.Key
		if len(s3.cfg.Prefix) != 0 {
			key = obj.Key[len(s3.cfg.Prefix)+1:]
		}
		keySplit := strings.Split(key, "/")
		if len(keySplit) < 4 {
			continue
		}
		t := vscode.VersionTag{
			UniqueID:       uid,
			Version:        keySplit[2],
			TargetPlatform: keySplit[3],
		}
		slog.Debug("found tag", "stringValue", t.String(), "key", obj.Key, "prefix", prefix)
		tags[t.String()] = t
	}
	tagarr := []vscode.VersionTag{}
	for _, v := range tags {
		tagarr = append(tagarr, v)
	}
	return tagarr, s3err
}

func (s3 S3Backend) LoadExtensionMetadata(uid vscode.UniqueID) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	obj, err := s3.c.GetObject(ctx, s3.bkt, filepath.Join(s3.cfg.Prefix, ExtensionPath(uid), extensionMetadataFilename), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	return io.ReadAll(obj)
}

func (s3 S3Backend) LoadVersionMetadata(tag vscode.VersionTag) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectName := filepath.Join(s3.cfg.Prefix, AssetPath(tag), versionMetadataFilename)
	obj, err := s3.c.GetObject(ctx, s3.bkt, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting object %v: %w", objectName, err)
	}

	return io.ReadAll(obj)
}

func (s3 S3Backend) SaveExtensionMetadata(ext vscode.Extension) error {
	ext.Versions = []vscode.Version{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jext := ext.ToJSON()
	objectName := filepath.Join(s3.cfg.Prefix, ExtensionPath(ext.UniqueID()), extensionMetadataFilename)

	_, err := s3.c.PutObject(ctx, s3.bkt, objectName, bytes.NewReader(jext), int64(len(jext)), minio.PutObjectOptions{
		ContentType: "application/json",
	})

	if s3.cfg.apcDelta {
		return s3.saveAPCDelta(ctx, objectName)
	}

	return err
}

func (s3 S3Backend) SaveVersionMetadata(uid vscode.UniqueID, v vscode.Version) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tag := v.Tag(uid)

	jv := v.ToJSON()
	objectName := filepath.Join(s3.cfg.Prefix, AssetPath(tag), versionMetadataFilename)

	_, err := s3.c.PutObject(ctx, s3.bkt, objectName, bytes.NewReader(jv), int64(len(jv)), minio.PutObjectOptions{
		ContentType: "application/json",
	})

	if s3.cfg.apcDelta {
		return s3.saveAPCDelta(ctx, objectName)
	}

	return err
}

func (s3 S3Backend) SaveAsset(tag vscode.VersionTag, atype vscode.AssetTypeKey, contentType string, data []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectName := filepath.Join(s3.cfg.Prefix, AssetPath(tag), string(atype))

	_, err := s3.c.PutObject(ctx, s3.bkt, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})

	if s3.cfg.apcDelta {
		return s3.saveAPCDelta(ctx, objectName)
	}

	return err
}

func (s3 S3Backend) Remove(tag vscode.VersionTag) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectName := s3.tagToKey(tag)
	if err := s3.c.RemoveObject(ctx, s3.bkt, objectName, minio.RemoveObjectOptions{ForceDelete: true}); err != nil {
		return err
	}

	tags, err := s3.ListVersionTags(tag.UniqueID)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		objectName := s3.uidToKey(tag.UniqueID)
		if err := s3.c.RemoveObject(ctx, s3.bkt, objectName, minio.RemoveObjectOptions{ForceDelete: true}); err != nil {
			return err
		}
	}
	return nil
}

func (s3 S3Backend) LoadAsset(tag vscode.VersionTag, atype vscode.AssetTypeKey) (io.ReadCloser, error) {
	return s3.c.GetObject(context.Background(), s3.bkt, filepath.Join(s3.cfg.Prefix, AssetPath(tag), versionMetadataFilename), minio.GetObjectOptions{})
}

func (s3 S3Backend) listPublishers() ([]string, error) {
	publishers := []string{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var s3err error
	prefix := s3.cfg.Prefix
	if len(prefix) != 0 {
		prefix = prefix + "/"
	}
	for obj := range s3.c.ListObjects(ctx, s3.bkt, minio.ListObjectsOptions{Prefix: prefix, Recursive: false}) {
		if obj.Err != nil {
			s3err = obj.Err
		} else {
			slog.Debug("publisher found", "publisher", filepath.Base(obj.Key), "prefix", prefix)
			publishers = append(publishers, filepath.Base(obj.Key))
		}
	}
	return publishers, s3err
}

func (s3 S3Backend) listUniqueID() ([]vscode.UniqueID, error) {
	publishers, err := s3.listPublishers()
	if err != nil {
		return nil, err
	}

	uids := map[string]vscode.UniqueID{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var s3err error
	for _, publisher := range publishers {
		for obj := range s3.c.ListObjects(ctx, s3.bkt, minio.ListObjectsOptions{Prefix: filepath.Join(s3.cfg.Prefix, publisher) + "/", Recursive: false}) {
			slog.Debug("extension name found", "key", obj.Key, "publisher", publisher, "path", obj.Key)
			spl := strings.Split(obj.Key, "/")
			if len(spl) > 2 {
				uid := vscode.UniqueID{Publisher: publisher, Name: filepath.Base(obj.Key)}
				uids[uid.String()] = uid
			}
			if obj.Err != nil {
				s3err = obj.Err
			}
		}
	}
	uidarr := []vscode.UniqueID{}
	for _, v := range uids {
		uidarr = append(uidarr, v)
	}
	return uidarr, s3err
}

func (s3 S3Backend) uidToKey(uid vscode.UniqueID) string {
	return filepath.Join(s3.cfg.Prefix, uid.Publisher, uid.Name)
}

func (s3 S3Backend) tagToKey(tag vscode.VersionTag) string {
	s := s3.uidToKey(tag.UniqueID)
	if tag.HasVersion() {
		s = filepath.Join(s, tag.Version)
		if tag.HasTargetPlatform() {
			s = filepath.Join(s, tag.TargetPlatform)
		}
	}
	return s
}

func (s3 S3Backend) saveAPCDelta(ctx context.Context, objectName string) error {
	deltaObjectName := filepath.Join("delta", time.Now().Format("2006_01_02"), objectName)
	data := []byte(objectName)
	slog.Debug("saving APC delta", "objectName", objectName, "deltaObjectName", deltaObjectName)
	_, err := s3.c.PutObject(ctx, s3.bkt, deltaObjectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "text/plain",
	})
	return err
}
