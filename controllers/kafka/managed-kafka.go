package kafka

import (
	corev1 "k8s.io/api/core/v1"
)

func NewManagedKafka(secret corev1.Secret) ManagedKafka {
	managedKafka := ManagedKafka{
		username:      string(secret.Data["client.id"]),
		password:      string(secret.Data["client.secret"]),
		hostname:      string(secret.Data["hostname"]),
		adminHostname: string(secret.Data["admin.url"]),
		tokenURL:      string(secret.Data["token.url"]),
	}

	return managedKafka
}

func (m *ManagedKafka) Get(path string) error {
	return nil
}

func (m *ManagedKafka) Post(path string) error {
	return nil
}

func (m *ManagedKafka) Delete(path string) error {
	return nil
}
