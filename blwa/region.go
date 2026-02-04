package blwa

// Region represents a target AWS region for client creation.
type Region interface {
	// resolve returns the AWS region string using the environment.
	resolve(env Environment) string
}

// localRegion uses the Lambda's AWS_REGION environment variable.
type localRegion struct{}

// resolve returns the AWS_REGION from the parsed environment.
func (localRegion) resolve(env Environment) string {
	return env.awsRegion()
}

// LocalRegion returns a Region that uses the Lambda's AWS_REGION.
func LocalRegion() Region {
	return localRegion{}
}

// primaryRegion uses the PRIMARY_REGION environment variable.
// This is set by the CDK construct to point to the primary deployment region.
type primaryRegion struct{}

// resolve returns the PRIMARY_REGION from the parsed environment.
func (primaryRegion) resolve(env Environment) string {
	return env.primaryRegion()
}

// PrimaryRegion returns a Region that uses the PRIMARY_REGION env var.
// Use this for cross-region operations that must target the primary deployment region.
func PrimaryRegion() Region {
	return primaryRegion{}
}

// fixedRegion uses a hardcoded region string.
type fixedRegion string

// resolve returns the fixed region string.
func (r fixedRegion) resolve(_ Environment) string {
	return string(r)
}

// FixedRegion returns a Region that uses a specific region string.
func FixedRegion(region string) Region {
	return fixedRegion(region)
}
