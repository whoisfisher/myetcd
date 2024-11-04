package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

func loggedError(log logr.Logger, err error, message string) error {
	log.Error(err, message)
	return fmt.Errorf("%s: %s", message, err)
}

func main() {
	var (
		backupTempDir          string
		etcdName               string
		etcdUrl                string
		caPath                 string
		keyPath                string
		certPath               string
		etcdDialTimeoutSeconds int64
		timeoutSeconds         int64
	)

	flag.StringVar(&backupTempDir, "backup-tmp-dir", os.TempDir(), "The directory to temporarily place backups before they are uploaded to their destination.")
	flag.StringVar(&etcdName, "etcd-name", "<etcd-master1>", "name for etcd.")
	flag.StringVar(&etcdUrl, "etcd-url", "<https://localhost:2379>", "url for etcd.")
	flag.StringVar(&caPath, "ca-path", "</etc/ssl/etcd/ssl/ca.pem>", "ca.pem for etcd.")
	flag.StringVar(&keyPath, "key-path", "</etc/ssl/etcd/ssl/admin-<nodename>-key.pem>", "key.pem for etcd.")
	flag.StringVar(&certPath, "cert-path", "</etc/ssl/etcd/ssl/admin-<nodename>.pem>", "cert.pem for etcd.")
	flag.Int64Var(&etcdDialTimeoutSeconds, "etcd-dial-timeout-seconds", 5, "timeout, inseconds, for dialing the etcd api.")
	flag.Int64Var(&timeoutSeconds, "timeout-seconds", 60, "timeout, in seconds, of whole restore operation.")
	flag.Parse()

	zapLogger := zap.NewRaw(zap.UseDevMode(true))
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	log := ctrl.Log.WithName("backup-agent")
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(timeoutSeconds))
	defer ctxCancel()

	etcd_config := clientv3.Config{
		Endpoints:   []string{etcdUrl}, // etcd 集群的地址
		DialTimeout: time.Duration(etcdDialTimeoutSeconds) * time.Second,
		Logger:      zapLogger.Named("etcd-client"),
	}

	log.Info("connection to etcd and getting snapshot")
	localPath := filepath.Join(backupTempDir, "snapshot.db")
	snap := snapshot.NewV3(zapLogger.Named("snapshot"))
	if err := snap.Save(ctx, etcd_config, localPath); err != nil {
		log.Info("Cannot create backup for etcd.")
	}
}
