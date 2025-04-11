package instancetemplate

func buildInstanceName2TemplateMap(itsExt *instanceSetExt) (map[string]*instanceTemplateExt, error) {
	instanceTemplateList := buildInstanceTemplateExts(itsExt)
	allNameTemplateMap := make(map[string]*instanceTemplateExt)
	var instanceNameList []string
	for _, template := range instanceTemplateList {
		ordinalList, err := GetOrdinalListByTemplateName(itsExt.its, template.Name)
		if err != nil {
			return nil, err
		}
		instanceNames, err := GenerateInstanceNamesFromTemplate(itsExt.its.Name, template.Name, template.Replicas, itsExt.its.Spec.OfflineInstances, ordinalList)
		if err != nil {
			return nil, err
		}
		instanceNameList = append(instanceNameList, instanceNames...)
		for _, name := range instanceNames {
			allNameTemplateMap[name] = template
		}
	}
	// validate duplicate pod names
	getNameFunc := func(n string) string {
		return n
	}
	if err := ValidateDupInstanceNames(instanceNameList, getNameFunc); err != nil {
		return nil, err
	}

	return allNameTemplateMap, nil
}
