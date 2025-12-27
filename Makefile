.PHONY: clean gen

# Clean all generated files
clean:
	@echo "Cleaning generated files..."
	rm -rf pulumi/pkg/istio/crds
	rm -f pulumi/pkg/istio/istio-crds.yaml

# Generate CRD types
gen: clean
	@echo "Generating CRD types..."
	cd pulumi && go generate ./...
