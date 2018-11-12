ARG stack=packs/v3

FROM ${stack}

COPY buildpacks /buildpacks

RUN mkdir -p /launch/app
RUN chown -R packs:packs /launch

ENV PACK_BP_PATH /buildpacks
ENV PACK_BP_ORDER_PATH /buildpacks/order.toml
ENV PACK_BP_GROUP_PATH ./group.toml
ENV PACK_DETECT_INFO_PATH ./detect.toml
ENV PACK_STACK_NAME ${stack}

USER packs

ENTRYPOINT ["/packs/detector"]
