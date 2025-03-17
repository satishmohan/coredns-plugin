PLUGIN_NAME = coredns-plugin
BUILD_DIR = build
APPIDENTIFY_DIR = appidentify

all: build

build:
	@echo "Building $(PLUGIN_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(PLUGIN_NAME) ./main.go

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	@go test ./$(APPIDENTIFY_DIR)/...

.PHONY: all build clean test
