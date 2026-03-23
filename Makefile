BUILD_TIME := $(shell date "+%F %T")
COMMIT_SHA1 := $(shell git rev-parse HEAD )

# 版本手动指定
VERSION=v1.1.0
LDFLAGS := -ldflags "-X 'github.com/jackz-jones/cross-chain-service/internal.BuildTime=${BUILD_TIME}'  -X 'github.com/jackz-jones/cross-chain-service/internal.CommitID=${COMMIT_SHA1}'  -X 'github.com/jackz-jones/cross-chain-service/internal.Version=${VERSION}'"
SOURCE := ./crosschain.go
BUILD_NAME := cross-chain-service

IMAGE=192.168.1.2:5000/cross-chain-service
REV=$(shell git rev-parse --short HEAD)
REPO=${IMAGE}-${REV}:${VERSION}
TAG=192.168.1.2:5000/cross-chain-service:${VERSION}

.PHONY:gen-code start-service build

gen-code:
	@sh ./scripts/generate_code.sh crosschain

start-service:
	go run ${SOURCE}

build:
	go build ${LDFLAGS} -o ${BUILD_NAME} ${SOURCE}

build-docker:
	GO111MODULE=on go mod vendor
	docker build -t ${REPO} -f ./docker/Dockerfile .
	docker tag ${REPO} ${TAG}
	docker push ${REPO}
	docker push ${TAG}

push-docker:
	docker push ${REPO}

gen-cert:
	bash ./scripts/generate_cert.sh ./cert ./scripts

gen-mock:
	mockgen -source=crosschain/crosschain.go -destination=mock/mock_cross_chain.go -package=mock

ut:
	#cd scripts && ./ut_cover.sh
	go test -coverprofile cover.out ./...
	@echo "\n"
	@echo "综合UT覆盖率：" `go tool cover -func=cover.out | tail -1  | grep -P '\d+\.\d+(?=\%)' -o`
	@echo "\n"

lint:
	golangci-lint run ./...

comment:
	gocloc --include-lang=Go --output-type=json --not-match=".*_test.go|types.go" --not-match-d="cert|chain|mock|pb|internal/code|internal/errors|internal/server|internal/model" . | jq '(.total.comment-.total.files*5)/(.total.code+.total.comment)*100'

pre-commit: lint ut comment

update-mod:
	go get github.com/jackz-jones/blockchain-interactive-service@dev
	go get github.com/jackz-jones/notification-contract-go@dev
	go get github.com/jackz-jones/nft-contract-go@dev
	go get github.com/ethereum/go-ethereum@v1.14.11
	go get github.com/jackz-jones/common@dev
	go mod tidy