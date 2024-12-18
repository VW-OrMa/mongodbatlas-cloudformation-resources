FROM golang:1.23-alpine

USER root
WORKDIR /asset-input

RUN apk update && apk --no-cache add make zip git pipx

RUN pipx install cloudformation-cli-go-plugin --include-deps && pipx inject cloudformation-cli-go-plugin setuptools
# Expose all pipx venv binaries globally
ENV PATH=$PATH:/root/.local/bin

COPY ./cfn-resources .
COPY .git ./.git

RUN mkdir -p /output
RUN for dir in ./*; do \
    if [ -d "$dir" ] && [ -f "$dir/Makefile" ] && [ -d "$dir/cmd" ]; then \
        echo "Processing $dir"; \
        cd "$dir" && \
        make --silent && \
        cd bin && \
        zip -X ../bootstrap.zip ./bootstrap && \
        cd .. && \
        cp mongodb-atlas-*.json schema.json && \
        zip -X /output/$(basename "$dir").zip ./bootstrap.zip ./schema.json ./.rpdk-config && \
        cd ..; \
        echo; \
    else \
        echo "Skipping $dir"; \
    fi; \
done

CMD cp /output/*.zip /asset-output/