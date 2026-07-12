.PHONY: clean gen sync

# Clean all generated files
clean:
	@echo "Cleaning generated files..."
	rm -rf pkg/crds/istio/crds
	rm -f pkg/crds/istio/istio-crds.yaml
	rm -rf pkg/crds/gatewayapi/crds
	rm -f pkg/crds/gatewayapi/gateway-api-crds.yaml

# Generate CRD types
gen: clean
	@echo "Generating CRD types..."
	go generate ./...

# Regenerate CRD types to match pkg/versions/versions.go - each gen-crds.sh
# reads its target version from there, so this is how you pick up a version
# bump made in versions.go.
sync: gen
