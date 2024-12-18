FROM golang:1.23 as build

RUN apt-get update && apt-get install -y zip

COPY . .

# Build with optional lambda.norpc tag
ENV GOOS=linux
ENV GOARCH=arm64
# CDK fromDockerBuild expects output in "/asset" directory
RUN go build -tags lambda.norpc -o /asset/bootstrap cmd/main.go