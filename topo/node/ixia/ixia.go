package ixia

import (
	"context"
	"encoding/json"

	topopb "github.com/google/kne/proto/topo"
	"github.com/google/kne/topo/node"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IxiaSpec struct {
	Config string `json:"config,omitempty"`
}

type Ixia struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IxiaSpec `json:"spec,omitempty"`
}

func New(pb *topopb.Node) (node.Implementation, error) {
	cfg := defaults(pb)
	proto.Merge(cfg, pb)
	node.FixServices(cfg)
	return &Node{
		pb: cfg,
	}, nil
}

type Node struct {
	pb *topopb.Node
}

func (n *Node) Proto() *topopb.Node {
	return n.pb
}

func (n *Node) CreateNodeResource(ctx context.Context, ni node.Interface) error {
	log.Infof("Create IxiaTG node resource %s", n.pb.Name)
	jsonConfig, err := json.Marshal(n.pb.Config)
	if err != nil {
		return err
	}
	newIxia := &Ixia{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "network.keysight.com/v1alpha1",
			Kind:       "IxiaTG",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.pb.Name,
			Namespace: ni.Namespace(),
		},
		Spec: IxiaSpec{
			Config: string(jsonConfig),
		},
	}
	body, err := json.Marshal(newIxia)
	if err != nil {
		return err
	}

	err = ni.KubeClient().CoreV1().RESTClient().
		Post().
		AbsPath("/apis/network.keysight.com/v1alpha1").
		Namespace(ni.Namespace()).
		Resource("Ixiatgs").
		Body(body).
		Do(ctx).
		Error()
	if err != nil {
		log.Error(err)
		return err
	}
	log.Infof("Created custom resource: %s", n.pb.Name)
	return nil
}

func (n *Node) DeleteNodeResource(ctx context.Context, ni node.Interface) error {
	log.Infof("Delete IxiaTG node resource %s", n.pb.Name)
	err := ni.KubeClient().CoreV1().RESTClient().
		Delete().
		AbsPath("/apis/network.keysight.com/v1alpha1").
		Namespace(ni.Namespace()).
		Resource("Ixiatgs").
		Name(n.pb.Name).
		Do(ctx).
		Error()
	if err != nil {
		log.Error(err)
		return err
	}
	log.Infof("Deleted custom resource: %s", n.pb.Name)
	return nil
}

func defaults(pb *topopb.Node) *topopb.Node {
	return &topopb.Node{}
}

func init() {
	node.Register(topopb.Node_IXIA_TG, New)
}
