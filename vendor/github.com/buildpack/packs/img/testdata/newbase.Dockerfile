FROM alpine

RUN mkdir /layers
RUN echo some-new-base-layer > /layers/some-new-base-layer.txt
