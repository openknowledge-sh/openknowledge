package okf

func Validate(root string) (Result, error) {
	return ValidateWithVersion(root, LatestSpecVersion)
}

func ValidateWithVersion(root string, version string) (Result, error) {
	result, _, err := parseAndValidateASTBundle(root, version)
	return result, err
}

func ValidateWithVersionAndOptions(root string, version string, options ValidationOptions) (Result, error) {
	result, _, err := parseAndValidateASTBundleWithOptions(root, version, options)
	return result, err
}
