package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/test"
	"gopkg.in/h2non/gock.v1"
	"time"
)

var _ = Describe("Pipeline operations", func() {
	var i *Iteration

	BeforeEach(func() {
		iteration, err := Before()
		Expect(err).ToNot(HaveOccurred())
		i = iteration
	})

	AfterEach(func() {
		err := After(i)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Kafka Connect", func() {
		It("Restarts Kafka Connect when /connectors is unreachable", func() {
			Skip("Broken")
			defer gock.Off()
			defer test.ForwardPorts()

			gock.New("http://connect-connect-api.test.svc:8083").
				Get("/connectors").
				Reply(500)

			originalPodName, err := i.getConnectPodName()
			Expect(err).ToNot(HaveOccurred())

			i.CreatePipeline()
			requeue, err := i.ReconcileKafkaConnect()
			Expect(requeue).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())

			time.Sleep(time.Second * 3) //give the old connect pod time to completely go away

			newPodName, err := i.getConnectPodName()
			Expect(err).ToNot(HaveOccurred())

			Expect(originalPodName).ToNot(Equal(newPodName))
		})

		It("Restarts Kafka Connect when /connectors/<connector> is unreachable", func() {
			Skip("Broken")
			defer gock.Off()
			defer test.ForwardPorts()

			originalPodName, err := i.getConnectPodName()
			Expect(err).ToNot(HaveOccurred())

			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())

			gock.New("http://connect-connect-api.test.svc:8083").
				Get("/connectors/" + pipeline.Status.ActiveDebeziumConnectorName).
				Reply(500)

			gock.New("http://connect-connect-api.test.svc:8083").
				Get("/connectors/" + pipeline.Status.ActiveESConnectorName).
				Reply(500)

			requeue, _ := i.ReconcileKafkaConnect()
			Expect(requeue).To(BeFalse())

			time.Sleep(time.Second * 3) //give the old connect pod time to completely go away

			newPodName, err := i.getConnectPodName()
			Expect(err).ToNot(HaveOccurred())

			Expect(originalPodName).ToNot(Equal(newPodName))
		})

		It("Doesn't restart Kafka Connect when it is available", func() {
			Skip("Broken")
			originalPodName, err := i.getConnectPodName()
			Expect(err).ToNot(HaveOccurred())
			i.CreatePipeline()
			requeue, err := i.ReconcileKafkaConnect()
			Expect(requeue).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
			newPodName, err := i.getConnectPodName()
			Expect(originalPodName).To(Equal(newPodName))
		})
	})
})
