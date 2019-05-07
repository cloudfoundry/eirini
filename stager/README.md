# Stager

The stager receives staging requests from the CloudController, translates them to an OPI (staging) task, and schedules the task on the scheduler underlying OPI. The code that is run by the task container is located in [eirini-staging](https://github.com/cloudfoundry-incubator/eirini-staging). It is basically responsible for downloading app-bits, staging, and uploading the resulting droplet (read more about the native staging process in [eirini-staging](https://github.com/cloudfoundry-incubator/eirini-staging)).
