ARG base
FROM $base

RUN echo upper-layer-1 > /layers/upper-layer-1.txt
RUN echo upper-layer-2 > /layers/upper-layer-2.txt
