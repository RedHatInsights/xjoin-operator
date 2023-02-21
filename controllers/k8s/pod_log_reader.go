package k8s

import (
	"bytes"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type LogReader interface {
	GetLogs(name string, namespace string) (string, error)
}

type PodLogReader struct {
	ClientSet kubernetes.Interface
}

func (p PodLogReader) GetLogs(name string, namespace string) (string, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	podLogOpts := v1.PodLogOptions{}
	req := p.ClientSet.CoreV1().Pods(namespace).GetLogs(name, &podLogOpts)

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	str := buf.String()
	return str, nil
}
