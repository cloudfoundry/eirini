FROM alpine

RUN mkdir /layers
RUN echo some-layer-1 > /layers/some-layer-1.txt
RUN echo some-layer-2 > /layers/some-layer-2.txt
RUN echo some-layer-3 > /layers/some-layer-3.txt
