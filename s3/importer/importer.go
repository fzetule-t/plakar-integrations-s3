/*
 * Copyright (c) 2023 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package importer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"

	"github.com/PlakarKorp/kloset/connectors"
	"github.com/PlakarKorp/kloset/connectors/importer"
	"github.com/PlakarKorp/kloset/location"
	"github.com/PlakarKorp/kloset/objects"
)

type S3Importer struct {
	minioClient *minio.Client

	bucket       string
	host         string
	scanDir      string
	includePaths []string
	ssec         encrypt.ServerSide
}

func init() {
	importer.Register("s3", 0, NewS3Importer)
}

func connect(endpoint string, useSsl, insecure bool, accessKeyID, secretAccessKey string) (*minio.Client, error) {
	transport, err := minio.DefaultTransport(useSsl)
	if err != nil {
		return nil, err
	}

	if useSsl && insecure {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure:    useSsl,
		Transport: transport,
	})
	if err != nil {
		return nil, err
	}

	client.SetAppInfo("plakar", "v1.1.0")

	return client, nil
}

func NewS3Importer(ctx context.Context, opts *connectors.Options, name string, config map[string]string) (importer.Importer, error) {
	target := config["location"]

	includePathsStr, ok := config["includePaths"]

	var includePaths []string
	if ok && includePathsStr != "" {
		for _, path := range strings.Split(includePathsStr, ",") {
			includePaths = append(includePaths, strings.TrimSpace(path))
		}
		log.Printf("NewS3Importer - includePaths: %v", includePaths)
	}

	var accessKey string
	if tmp, ok := config["access_key"]; !ok {
		return nil, fmt.Errorf("missing access_key")
	} else {
		accessKey = tmp
	}

	var secretAccessKey string
	if tmp, ok := config["secret_access_key"]; !ok {
		return nil, fmt.Errorf("missing secret_access_key")
	} else {
		secretAccessKey = tmp
	}

	useSsl := true
	if value, ok := config["use_tls"]; ok {
		tmp, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid use_tls value")
		}
		useSsl = tmp
	}

	insecure := false
	if value, ok := config["tls_insecure_no_verify"]; ok {
		tmp, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid tls_insecure_no_verify value")
		}
		insecure = tmp
	}

	virtualHost := false
	if value, ok := config["virtual_host"]; ok {
		tmp, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid virtual_host value")
		}
		virtualHost = tmp
	}

	endpoint := config["endpoint"]

	var port string
	if tmp, ok := config["port"]; ok {
		port = tmp
	}

	var root string
	if tmp, ok := config["root"]; ok {
		root = tmp
	}

	var ssec encrypt.ServerSide
	if value, ok := config["sse_customer_key"]; ok && value != "" {
		keyBytes, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("invalid sse_customer_key: must be base64-encoded: %w", err)
		}
		ssec, err = encrypt.NewSSEC(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid sse_customer_key: %w", err)
		}
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	if parsed.Port() == "" && port != "" {
		parsed.Host = fmt.Sprintf("%s:%s", parsed.Host, port)
	}

	var bucket, scanDir, host string
	if virtualHost {
		if endpoint == "" {
			return nil, fmt.Errorf("missing endpoint when virtual_host=true")
		}

		bucket, host, err = SplitVirtualHost(parsed.Host, endpoint)
		if err != nil {
			return nil, err
		}

		scanDir = strings.TrimPrefix(parsed.Path, "/")
	} else {
		if endpoint != "" {
			if parsed.Host != endpoint {
				parsed.Path = path.Join(parsed.Host, parsed.Path)
				parsed.Host = endpoint
			}
		}
		host = parsed.Host

		source := parsed.Path
		if root != "" {
			source = root
		}
		bucket, scanDir, _ = strings.Cut(strings.TrimPrefix(source, "/"), "/")
	}

	if bucket == "" || host == "" {
		return nil, fmt.Errorf("failed to parse the location: bucket name or host name are empty")
	}

	if !strings.HasPrefix(scanDir, "/") {
		scanDir = "/" + scanDir
	}

	conn, err := connect(host, useSsl, insecure, accessKey, secretAccessKey)
	if err != nil {
		return nil, err
	}

	return &S3Importer{
		bucket:       bucket,
		scanDir:      scanDir,
		includePaths: includePaths,
		minioClient:  conn,
		host:         host,
		ssec:         ssec,
	}, nil
}

func (p *S3Importer) Root() string          { return p.scanDir }
func (p *S3Importer) Origin() string        { return path.Join(p.host, p.bucket) }
func (p *S3Importer) Type() string          { return "s3" }
func (p *S3Importer) Flags() location.Flags { return 0 }

func (p *S3Importer) Ping(ctx context.Context) error {
	ok, err := p.minioClient.BucketExists(ctx, p.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket does not exist")
	}
	return nil
}

func (p *S3Importer) Import(ctx context.Context, records chan<- *connectors.Record, results <-chan *connectors.Result) error {
	defer close(records)
	log.Printf("S3Exporter.Import")

	const statWorkers = 16

	type statJob struct {
		key string
		fi  objects.FileInfo
	}

	jobs := make(chan statJob, statWorkers*2)

	var workers sync.WaitGroup

	// StatObject worker pool.
	for range statWorkers {
		workers.Add(1)

		go func() {
			defer workers.Done()

			for job := range jobs {
				// Stop accepting more work if the context was cancelled.
				select {
				case <-ctx.Done():
					return
				default:
				}

				var xattr []string
				var recordErr error

				stat, statErr := p.minioClient.StatObject(
					ctx,
					p.bucket,
					job.key,
					minio.StatObjectOptions{
						ServerSideEncryption: p.ssec,
					},
				)

				if statErr != nil {
					log.Printf("StatObject FAILED key=%s err=%v", job.key, statErr)
					recordErr = statErr
				} else if xattrBytes, marshalErr := json.Marshal(stat); marshalErr == nil {
					log.Printf("StatObject OK key=%s", job.key)
					xattr = append(xattr, string(xattrBytes))
				}

				key := job.key
				recordErrForGet := recordErr

				record := connectors.NewRecord(
					"/"+key,
					"",
					job.fi,
					xattr,
					func() (io.ReadCloser, error) {
						if recordErrForGet != nil {
							return nil, recordErrForGet
						}

						return p.minioClient.GetObject(
							ctx,
							p.bucket,
							key,
							minio.GetObjectOptions{
								ServerSideEncryption: p.ssec,
							},
						)
					},
				)

				// Send the record as soon as this StatObject finishes.
				select {
				case records <- record:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	prefixes := p.includePaths
	if len(prefixes) == 0 {
		prefixes = []string{p.scanDir}
	}

	var listErr error

	for _, prefix := range prefixes {
		prefix = strings.Trim(prefix, "/")
		if prefix != "" {
			prefix += "/"
		}

		listopts := minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		}
		log.Printf("ListObjectsOptions.prefix: %s, recursive: %t", listopts.Prefix, listopts.Recursive)

		for object := range p.minioClient.ListObjects(ctx, p.bucket, listopts) {
			if object.Err != nil {
				listErr = object.Err
				continue
			}

			if strings.HasSuffix(object.Key, "/") {
				continue
			}

			key := object.Key

			fi := objects.FileInfo{
				Lname:    path.Base("/" + key),
				Lsize:    object.Size,
				Lmode:    0o700,
				LmodTime: object.LastModified,
				Ldev:     1,
			}

			select {
			case jobs <- statJob{
				key: key,
				fi:  fi,
			}:
			case <-ctx.Done():
				close(jobs)
				workers.Wait()
				return ctx.Err()
			}
		}
	}

	close(jobs)
	workers.Wait()

	if listErr != nil {
		return listErr
	}

	return nil
}

func (p *S3Importer) Close(ctx context.Context) error {
	return nil
}
