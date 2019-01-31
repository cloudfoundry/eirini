package models

func (m *MetricTagValue) Validate() error {
	var validationError ValidationError

	if m.Static != "" && m.Dynamic.Valid() {
		validationError = validationError.Append(ErrInvalidField{"static"})
		validationError = validationError.Append(ErrInvalidField{"dynamic"})
	}

	if m.Static == "" && !m.Dynamic.Valid() {
		validationError = validationError.Append(ErrInvalidField{"static"})
		validationError = validationError.Append(ErrInvalidField{"dynamic"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (v MetricTagValue_DynamicValue) Valid() bool {
	switch v {
	case MetricTagDynamicValueIndex:
		return true
	case MetricTagDynamicValueInstanceGuid:
		return true
	default:
		return false
	}
}

func validateMetricTags(m map[string]*MetricTagValue, metricsGuid string) ValidationError {
	var validationError ValidationError

	for _, v := range m {
		err := v.Validate()
		if err != nil {
			validationError = validationError.Append(err)
		}
	}

	if len(m) > 0 && metricsGuid != "" {
		if source_id, ok := m["source_id"]; !ok || source_id.Static != metricsGuid {
			validationError = validationError.Append(ErrInvalidField{`source_id should match metrics_guid`})
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}
