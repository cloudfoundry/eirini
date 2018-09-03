FROM packs/cf:build as builder

COPY . .

RUN /packs/builder \
  -buildpacksDir /var/lib/buildpacks \
  -outputDroplet /out/droplet.tgz \
  -outputBuildArtifactsCache /dev/null \
  -outputMetadata /out/result.json \

FROM packs/cf:run

ADD --from=builder /out/droplet.tgz /home/vcap

ENTRYPOINT ["/packs/launcher"]