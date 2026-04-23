.ONESHELL:
PRODUCT_NAME=edgegate-core
BASENAME=$(PRODUCT_NAME)
BINDIR=bin
LIBNAME=$(PRODUCT_NAME)
CLINAME=EdgegateCli

BRANCH=$(shell git branch --show-current)
VERSION=$(shell git describe --tags || echo "unknown version")
ifeq ($(OS),Windows_NT)
# Not available for Windows! use bash in WSL
endif

TAGS=with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api
IOS_ADD_TAGS=with_dhcp,with_low_memory,with_conntrack
ANDROID_LDFLAGS=-w -s -linkmode=external -extldflags=-Wl,-z,max-page-size=16384
DESKTOP_LDFLAGS=-w -s
GOBUILDLIB=CGO_ENABLED=1 go build -trimpath -tags $(TAGS) -ldflags="$(DESKTOP_LDFLAGS)" -buildmode=c-shared
GOBUILDSRV=CGO_ENABLED=1 go build -ldflags "-s -w" -trimpath -tags $(TAGS)
WINDOWS_MINGW_PATH=D:/msys64/mingw64/bin:D:/msys64/usr/bin:$$PATH
SKIP_NPM ?=
SYNC_SCRIPT=../sync-edgegates-core-artifacts.ps1
UNAME_S := $(shell uname -s 2>/dev/null || echo unknown)
ifneq (,$(filter Windows_NT,$(OS)))
SKIP_NPM := 1
endif
ifneq (,$(findstring MINGW,$(UNAME_S)))
SKIP_NPM := 1
endif
ifneq (,$(findstring MSYS,$(UNAME_S)))
SKIP_NPM := 1
endif
ifneq (,$(findstring CYGWIN,$(UNAME_S)))
SKIP_NPM := 1
endif

.PHONY: protos
protos:
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
	# protoc --go_out=./ --go-grpc_out=./ --proto_path=edgegate-core-rpc edgegate-core-rpc/*.proto
	# for f in $(shell find v2 -name "*.proto"); do \
	# 	protoc --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --go_out=./ --go-grpc_out=./  $$f; \
	# done
	# for f in $(shell find extension -name "*.proto"); do \
	# 	protoc --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --go_out=./ --go-grpc_out=./  $$f; \
	# done
	protoc --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --go_out=./ --go-grpc_out=./  $(shell find v2 -name "*.proto") $(shell find extension -name "*.proto")
	protoc --doc_out=./docs  --doc_opt=markdown,edgegate-core-rpc.md $(shell find v2 -name "*.proto") $(shell find extension -name "*.proto")
	# protoc --js_out=import_style=commonjs,binary:./extension/html/rpc/ --grpc-web_out=import_style=commonjs,mode=grpcwebtext:./extension/html/rpc/ $(shell find v2 -name "*.proto") $(shell find extension -name "*.proto")
	# npx browserify extension/html/rpc/extension.js >extension/html/rpc.js


lib_install: prepare
	go install -v github.com/sagernet/gomobile/cmd/gomobile@v0.1.12
	go install -v github.com/sagernet/gomobile/cmd/gobind@v0.1.12
ifneq ($(SKIP_NPM),1)
	npm install
else
	@echo "Skip npm install on Windows (SKIP_NPM=1)"
endif

headers:
	go build -buildmode=c-archive -o $(BINDIR)/ ./platform/desktop2

android: lib_install
	gomobile bind -v -androidapi=21 -javapkg=com.edgegate.core -libname=edgegate-core -tags=$(TAGS) -trimpath -target=android -ldflags="$(ANDROID_LDFLAGS)" -o $(BINDIR)/$(LIBNAME).aar github.com/sagernet/sing-box/experimental/libbox ./platform/mobile

sync-artifacts:
	@if [ -f "$(SYNC_SCRIPT)" ]; then \
		if command -v pwsh >/dev/null 2>&1; then \
			pwsh -NoProfile -File "$(SYNC_SCRIPT)"; \
		elif command -v powershell >/dev/null 2>&1; then \
			powershell -NoProfile -ExecutionPolicy Bypass -File "$(SYNC_SCRIPT)"; \
		else \
			echo "Skip sync: neither pwsh nor powershell found"; \
		fi; \
	else \
		echo "Skip sync: script not found ($(SYNC_SCRIPT))"; \
	fi

android-sync: android sync-artifacts

ios-full: lib_install
	gomobile bind -v  -target ios,iossimulator,tvos,tvossimulator,macos -libname=edgegate-core -tags=$(TAGS),$(IOS_ADD_TAGS) -trimpath -ldflags="-w -s" -o $(BINDIR)/$(PRODUCT_NAME).xcframework github.com/sagernet/sing-box/experimental/libbox ./platform/mobile && \
	mv $(BINDIR)/$(PRODUCT_NAME).xcframework $(BINDIR)/$(LIBNAME).xcframework 

ios: lib_install
	gomobile bind -v  -target ios -libname=edgegate-core -tags=$(TAGS),$(IOS_ADD_TAGS) -trimpath -ldflags="-w -s" -o $(BINDIR)/EdgegateCore.xcframework github.com/sagernet/sing-box/experimental/libbox ./platform/mobile && \
	cp Info.plist $(BINDIR)/EdgegateCore.xcframework/


webui:
	curl -L -o webui.zip  https://github.com/reddts/Yacd-meta/archive/gh-pages.zip 
	unzip -d ./ -q webui.zip
	rm webui.zip
	rm -rf bin/webui
	mv Yacd-meta-gh-pages bin/webui

.PHONY: build
windows-amd64: prepare
	cmd /c build_windows.bat

windows-amd64-sync: windows-amd64 sync-artifacts
	

linux-amd64: prepare
	mkdir -p $(BINDIR)/lib
	env GOOS=linux GOARCH=amd64 $(GOBUILDLIB) -o $(BINDIR)/lib/$(LIBNAME).so ./platform/desktop
	mkdir lib
	cp $(BINDIR)/lib/$(LIBNAME).so ./lib/$(LIBNAME).so
	env GOOS=linux GOARCH=amd64  CGO_LDFLAGS="./lib/$(LIBNAME).so" $(GOBUILDSRV) -o $(BINDIR)/$(CLINAME) ./cmd/bydll
	rm -rf ./lib
	chmod +x $(BINDIR)/$(CLINAME)
	make webui
linux-arm64:
	mkdir -p $(BINDIR)/lib
	env GOOS=linux GOARCH=arm64 $(GOBUILDLIB) -o $(BINDIR)/lib/$(LIBNAME).so ./platform/desktop
	mkdir lib
	cp $(BINDIR)/lib/$(LIBNAME).so ./lib/$(LIBNAME).so
	env GOOS=linux GOARCH=arm64  CGO_LDFLAGS="./lib/$(LIBNAME).so" $(GOBUILDSRV) -o $(BINDIR)/$(CLINAME) ./cmd/bydll
	rm -rf ./lib
	chmod +x $(BINDIR)/$(CLINAME)
	make webui


linux-custom: prepare
	mkdir -p $(BINDIR)/
	#env GOARCH=mips $(GOBUILDSRV) -o $(BINDIR)/$(CLINAME) ./cmd/
	go build -ldflags "-s -w" -trimpath -tags $(TAGS) -o $(BINDIR)/$(CLINAME) ./cmd/main
	chmod +x $(BINDIR)/$(CLINAME)
	make webui

macos-amd64:
	env GOOS=darwin GOARCH=amd64 CGO_CFLAGS="-mmacosx-version-min=10.11 -O2" CGO_LDFLAGS="-mmacosx-version-min=10.11 -O2 -lpthread" CGO_ENABLED=1 go build -trimpath -tags $(TAGS),$(IOS_ADD_TAGS) -buildmode=c-shared -o $(BINDIR)/$(LIBNAME)-amd64.dylib ./platform/desktop
macos-arm64:
	env GOOS=darwin GOARCH=arm64 CGO_CFLAGS="-mmacosx-version-min=10.11 -O2" CGO_LDFLAGS="-mmacosx-version-min=10.11 -O2 -lpthread" CGO_ENABLED=1 go build -trimpath -tags $(TAGS),$(IOS_ADD_TAGS) -buildmode=c-shared -o $(BINDIR)/$(LIBNAME)-arm64.dylib ./platform/desktop
	
macos: prepare macos-amd64 macos-arm64 
	
	lipo -create $(BINDIR)/$(LIBNAME)-amd64.dylib $(BINDIR)/$(LIBNAME)-arm64.dylib -output $(BINDIR)/$(LIBNAME).dylib
	cp $(BINDIR)/$(LIBNAME).dylib ./$(LIBNAME).dylib 
	mv $(BINDIR)/$(LIBNAME)-arm64.h $(BINDIR)/desktop.h 
	# env GOOS=darwin GOARCH=amd64 CGO_CFLAGS="-mmacosx-version-min=10.15" CGO_LDFLAGS="-mmacosx-version-min=10.15" CGO_LDFLAGS="bin/$(LIBNAME).dylib"  CGO_ENABLED=1 $(GOBUILDSRV)  -o $(BINDIR)/$(CLINAME) ./cmd/bydll
	# rm ./$(LIBNAME).dylib
	# chmod +x $(BINDIR)/$(CLINAME)

prepare: 
	go mod tidy

clean:
	rm $(BINDIR)/*




release: # Create a new tag for release.	
	@bash -c '.github/change_version.sh'
	


