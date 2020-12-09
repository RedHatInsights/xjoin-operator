package v1alpha1

func (instance *XJoinPipeline) GetUIDString() string {
	return string(instance.GetUID())
}
