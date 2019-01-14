# Stager

The stager receives staging requests from the CloudController, translates them to an OPI (staging) task, and schedules the task on the scheduler underlying OPI. The code that is run by the task container is located in `eirini/recipe`. It is basically responsible for downloading app-bits, staging, and uploading the resulting droplet (read more about the recipe job in `eirini/recipe/README.md`).

