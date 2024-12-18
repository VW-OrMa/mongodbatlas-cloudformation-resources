FROM golang:1.23 as build
#WORKDIR /vws-service-provider-handler
RUN apt-get update && apt-get install -y zip
# Copy dependencies list
COPY go.mod go.sum ./
COPY handler/ ./handler

# Build with optional lambda.norpc tag
ENV GOOS=linux
ENV GOARCH=arm64
# CDK fromDockerBuild expects output in "/asset" directory
RUN go build -tags lambda.norpc -o /asset/bootstrap handler/main.go