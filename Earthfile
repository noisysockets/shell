VERSION 0.7
FROM golang:1.22-bookworm
WORKDIR /workspace

tidy:
  BUILD +tidy-go
  BUILD +lint-ts

lint:
  BUILD +lint-go
  BUILD +lint-ts

build:
  BUILD +build-ts

test:
  BUILD +test-go
  BUILD +test-ts

tidy-go:
  LOCALLY
  RUN go mod tidy
  RUN go fmt ./...

lint-go:
  FROM golangci/golangci-lint:v1.57.2
  WORKDIR /workspace
  COPY . .
  RUN golangci-lint run --timeout 5m ./...

test-go:
  FROM +sources-go
  RUN go test -coverprofile=coverage.out -v ./...
  SAVE ARTIFACT coverage.out AS LOCAL coverage.out

dev-server:
  FROM +sources-go
  RUN mkdir -p ./bin
  RUN go build -o ./bin/server ./server/main.go
  SAVE ARTIFACT ./bin/server

sources-go:
  COPY go.mod go.sum .
  RUN go mod download
  COPY . .

tidy-ts:
  FROM +sources-ts
  RUN npm run format
  SAVE ARTIFACT src AS LOCAL typescript/src

lint-ts:
  FROM +sources-ts
  RUN npm run lint

test-ts:
  FROM +build-ts
  COPY +dev-server/server ./bin/server
  RUN (./bin/server &) && npm run test

build-ts:
  FROM +sources-ts
  RUN npm run build

publish-ts:
  FROM +build-ts
  ARG VERSION
  RUN --secret NPM_TOKEN=npm_token echo "//registry.npmjs.org/:_authToken=${NPM_TOKEN}" > ~/.npmrc \
    && npm version ${VERSION} \
    && npm publish

sources-ts:
  FROM +tools
  COPY typescript/package.json typescript/package-lock.json ./typescript/
  WORKDIR /workspace/typescript
  RUN npm install
  COPY typescript ./

tools:
  RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  RUN apt install -y nodejs 