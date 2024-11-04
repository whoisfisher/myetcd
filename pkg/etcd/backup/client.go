package backup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-logr/zapr"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

var (
	DefaultCaPath                       = "/etc/ssl/etcd/ssl/ca.pem"
	DefaultCertPath                     = "/etc/ssl/etcd/ssl/admin-%s.pem"
	DefaultKeyPath                      = "/etc/ssl/etcd/ssl/admin-%s-key.pem"
	DefaultDataDir                      = "/var/lib/etcd"
	DefaultInitalClusteToken            = "k8s_etcd"
	DefaultPeerPort                     = 2380
	DefaultPort                         = 2379
	DefaultEtcdDialTimeoutSeconds int64 = 5
	DefaultTimeoutSeconds         int64 = 60
)

type EtcdClient struct {
	BackupTempDir          string
	EtcdDialTimeoutSeconds int64
	TimeoutSeconds         int64
	Log                    *zap.Logger
	Nodes                  []EtcdNode
}

type EtcdNode struct {
	Node                Node
	EtcdName            string
	CaCert              string
	KeyPath             string
	CertPath            string
	Endpoints           string
	PeerUrl             string
	InitialCluster      string
	InitialClusterToken string
	DataDir             string
}

type Node struct {
	Name    string
	Address string
}

func New(backupTempDir string, etcdDialTimeoutSeconds, timeoutSeconds int64, nodes []Node) *EtcdClient {
	zapLogger := logzap.NewRaw(logzap.UseDevMode(true))
	ctrl.SetLogger(zapr.NewLogger(zapLogger))
	EtcdNodes := []EtcdNode{}
	initialClusterStr := ""
	for i, member := range nodes {
		initialClusterStr += fmt.Sprintf("etcd-%s=https://%s:%s", member.Name, member.Address, DefaultPeerPort)
		if i < len(nodes)-1 {
			initialClusterStr += ","
		}
	}
	for _, node := range nodes {
		etcdNode := EtcdNode{}
		etcdNode.EtcdName = fmt.Sprintf("%s-%s", "etcd", node.Name)
		etcdNode.CaCert = DefaultCaPath
		etcdNode.CertPath = fmt.Sprintf(DefaultCertPath, "admin", node.Name)
		etcdNode.KeyPath = fmt.Sprintf(DefaultKeyPath, "admin", node.Name)
		etcdNode.PeerUrl = fmt.Sprintf("https://%s:%s", node.Address, DefaultPeerPort)
		etcdNode.Endpoints = fmt.Sprintf("%s:%s", node.Address, DefaultPort)
		etcdNode.InitialCluster = initialClusterStr
		etcdNode.InitialClusterToken = DefaultInitalClusteToken
		EtcdNodes = append(EtcdNodes, etcdNode)
	}

	return &EtcdClient{
		BackupTempDir:          backupTempDir,
		EtcdDialTimeoutSeconds: etcdDialTimeoutSeconds,
		TimeoutSeconds:         timeoutSeconds,
		Log:                    zapLogger,
		Nodes:                  EtcdNodes,
	}
}

func Default(backupTempDir string, nodes []Node) *EtcdClient {
	zapLogger := logzap.NewRaw(logzap.UseDevMode(true))
	ctrl.SetLogger(zapr.NewLogger(zapLogger))
	EtcdNodes := []EtcdNode{}
	initialClusterStr := ""
	for i, member := range nodes {
		initialClusterStr += fmt.Sprintf("etcd-%s=https://%s:%s", member.Name, member.Address, DefaultPeerPort)
		if i < len(nodes)-1 {
			initialClusterStr += ","
		}
	}
	for _, node := range nodes {
		etcdNode := EtcdNode{}
		etcdNode.EtcdName = fmt.Sprintf("%s-%s", "etcd", node.Name)
		etcdNode.CaCert = DefaultCaPath
		etcdNode.CertPath = fmt.Sprintf(DefaultCertPath, "admin", node.Name)
		etcdNode.KeyPath = fmt.Sprintf(DefaultKeyPath, "admin", node.Name)
		etcdNode.PeerUrl = fmt.Sprintf("https://%s:%s", node.Address, DefaultPeerPort)
		etcdNode.Endpoints = fmt.Sprintf("%s:%s", node.Address, DefaultPort)
		etcdNode.InitialCluster = initialClusterStr
		etcdNode.InitialClusterToken = DefaultInitalClusteToken
		EtcdNodes = append(EtcdNodes, etcdNode)
	}

	return &EtcdClient{
		BackupTempDir:          backupTempDir,
		EtcdDialTimeoutSeconds: DefaultEtcdDialTimeoutSeconds,
		TimeoutSeconds:         DefaultTimeoutSeconds,
		Log:                    zapLogger,
		Nodes:                  EtcdNodes,
	}
}

func (b *EtcdClient) Backup() error {
	etcd_config := clientv3.Config{
		Endpoints:   []string{b.Nodes[0].Endpoints},
		DialTimeout: time.Duration(b.EtcdDialTimeoutSeconds) * time.Second,
		Logger:      b.Log.Named("backup-agent"),
	}
	tlsConfig := b.Nodes[0].TLSConfig()
	if tlsConfig != nil {
		etcd_config.TLS = tlsConfig
	}
	b.Log.Info("starting etcd backup.")
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(b.TimeoutSeconds))
	defer ctxCancel()
	localPath := filepath.Join(b.BackupTempDir, "snapshot.db")
	snap := snapshot.NewV3(b.Log.Named("snapshot"))
	if err := snap.Save(ctx, etcd_config, localPath); err != nil {
		b.Log.Fatal("failed to create etcd snapshot.")
		return err
	}
	b.Log.Info("successfully to backup etcd.")
	return nil
}

func (b *EtcdClient) Restore() error {
	b.Log.Info("starting etcd restore.")
	localPath := filepath.Join(b.BackupTempDir, "snapshot.db")
	restoreConfig := snapshot.RestoreConfig{
		SnapshotPath:        localPath,
		Name:                b.Nodes[0].EtcdName,
		OutputDataDir:       DefaultDataDir,
		InitialCluster:      b.Nodes[0].InitialCluster,
		InitialClusterToken: DefaultInitalClusteToken,
		SkipHashCheck:       false,
	}
	snap := snapshot.NewV3(b.Log.Named("snapshot"))
	if err := snap.Restore(restoreConfig); err != nil {
		b.Log.Fatal("failed to restore etcd from snapshot")
		return err
	}
	b.Log.Info("successfully restored etcd from snapshot")
	for i := 1; i < len(b.Nodes); i++ {
		err := b.StartNode(i)
		if err != nil {
			b.Log.Error("failed to start etcd node", zap.Int("node", i+1))
			return err
		}
	}
	b.Log.Info("successfully started all etcd nodes")
	return nil
}

func (b *EtcdClient) StartNode(nodeIndex int) error {
	b.Log.Info("Starting node", zap.Int("nodeIndex", nodeIndex+1))
	clientConfig := clientv3.Config{
		Endpoints:   []string{b.Nodes[nodeIndex].Endpoints},
		DialTimeout: time.Second * time.Duration(b.EtcdDialTimeoutSeconds),
		TLS:         b.Nodes[nodeIndex].TLSConfig(),
	}
	client, err := clientv3.New(clientConfig)
	if err != nil {
		b.Log.Fatal("failed to create etcd client")
		return err
	}
	defer client.Close()
	_, err = client.MemberAdd(context.Background(), []string{b.Nodes[nodeIndex].PeerUrl})
	if err != nil {
		b.Log.Fatal("failed to add new etcd member to cluster")
		return err
	}
	return nil
}

func (b *EtcdClient) CreateClient(nodeIndex int) (*clientv3.Client, error) {
	etcd_config := clientv3.Config{
		Endpoints:   []string{b.Nodes[nodeIndex].Endpoints},
		DialTimeout: time.Duration(DefaultEtcdDialTimeoutSeconds) * time.Second,
		Logger:      b.Log.Named("etcd-client"),
	}
	tlsConfig := b.Nodes[nodeIndex].TLSConfig()
	if tlsConfig != nil {
		etcd_config.TLS = tlsConfig
	}
	b.Log.Info("creating etcd client")
	cli, err := clientv3.New(etcd_config)
	if err != nil {
		b.Log.Fatal("failed to create etcd client")
		return nil, err
	}
	return cli, nil
}

func (o *EtcdNode) TLSConfig() *tls.Config {
	caCert, err := os.ReadFile(o.CaCert)
	if err != nil {
		return nil
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil
	}
	clientCert, err := tls.LoadX509KeyPair(o.CertPath, o.KeyPath)
	if err != nil {
		return nil
	}
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{clientCert},
	}
	return tlsConfig
}
