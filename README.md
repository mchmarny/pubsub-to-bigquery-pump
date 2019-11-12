# Drain PubSub topic messages to BigQuery table

Simple utility to drain JSON messages on PubSub topic into BigQuery table.

## How to Use It

By combining thus [Cloud Run](https://cloud.google.com/run/) service and Cloud Scheduler configure multiple "import job" at specific interval.

![](./image/overview.png)

## Why Custom Service

Google Cloud has an easy approach to draining your PubSub messages into BigQuery. Using provided template you create a job that will consistently and reliably stream your messages into BigQuery.

```shell
gcloud dataflow jobs run $JOB_NAME --region us-west1 \
  --gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
  --parameters "inputTopic=projects/${PROJECT}/topics/${TOPIC},outputTableSpec=${PROJECT}:${DATASET}.${TABLE}"
```

This approach solves many of the common issues related to back pressure, retries, and individual insert quota limits. If you are either dealing with a constant stream of messages or need to drain your PubSub messages immediately into BigQuery, this is your best option.

The one downside of that approach is that, behind the scene, Dataflow deploys VMs. While the machine types and the number of VMs are configurable, there will always be at least one VM. That means that, whether there are messages to process or not, you always pay for VMs.

However, if your message flow is in-frequent or don't mind messages being written in scheduled batches, you can avoid that cost by using this service.

## Prerequisites

If you don't have one already, start by creating new project and configuring [Google Cloud SDK](https://cloud.google.com/sdk/docs/). Similarly, if you have not done so already, you will have [set up Cloud Run](https://cloud.google.com/run/docs/setup).

## Usage

> To keep this document short, I scripted longer `gcloud` commands. You should review these scripts for content and to understand the individual commands.

### Enable APIs

To start, you will need to enable a few GCP APIs

```shell
bin/apis
```

### Configure IAM

The deployed Cloud Run service will be follow the [principle of least privilege](https://searchsecurity.techtarget.com/definition/principle-of-least-privilege-POLP) (POLP) to ensure that both Cloud Scheduler and Cloud Run services have only the necessary rights and nothing more, we are going to create `pump-service` service accounts which will be used as Cloud Run and Cloud Scheduler service account identity.

```shell
bin/account
```

Now that we have the service account created, we can assign it the necessary policies:

* `run.invoker` - required to execute Cloud Run service
* `pubsub.subscriber` - required to list and read from Cloud PubSub subscription
* `bigquery.dataOwner` - required to write/read to BigQuery table
* `logging.logWriter` - required for Stackdriver logging
* `cloudtrace.agent` - required for Stackdriver tracing
* `monitoring.metricWriter` - required to write custom metrics to Stackdriver

To grant `pump-service` account all these IAM roles run:

```shell
bin/policy
```

## Deploying Service

Now that IAM is configured, we can deploy Cloud Run service

> By default we will deploy a prebuilt image (`gcr.io/cloudylabs-public/pubsub-to-bigquery-pump`). If you want to build this service from source, see [Building Image](#building-image) for instructions

```shell
bin/deploy
```

A couple of worth to note deployment options. First, we deployed the `pubsub-to-bigquery-pump` service to Cloud Run with `--no-allow-unauthenticated` flag which requires the invoking identity to have the `run.invoker` role. Unauthenticated requests will be rejected BEFORE they activating your service, that means no charge for unauthenticated request attempts. The service is also deployed with `--service-account` argument which will cause the `pubsub-to-bigquery-pump` service to run under the `pump-service` service account identity.

### Cloud Schedule

With our Cloud Run service deployed, we can now configure individual jobs in Cloud Scheduler that will execute your Cloud Run service.

> This service assumes that your BigQuery schema matches the names of JSON message fields. Column names are not case sensitive and the types conversion is best effort. You can use [this service](https://bigquery-json-schema-generator.com/) to generate BigQuery schema from the JSON in one of your PubSub messages

To configure job to drain your messages accumulated in PubSub topic to BigQuery create a job configuration file with this shape (see sample in [job/sample.json](./job/sample.json)):

```json
{
    "id": "my-import-job",
    "source": {
        "subscription": "my-iot-topic",
        "max_stall": 15
    },
    "target": {
        "dataset": "iot",
        "table": "events",
        "batch_size": 1000,
        "ignore_unknowns": true
    },
    "max_duration": 600
}
```

The above configuration defines:

* `id` is the unique ID for this job. Will be used in metrics to track the counts across executions
* `source` is the PubSub configuration
  * `subscription` is the name of existing PubSub subscription
  * `max_stall` represents the maximum amount of time (seconds) the service will wait for new messages when the queue has been drained. Should be greater than 5 seconds
* `target` is the BigQuery configuration
  * `dataset` is the name of the existing BigQuery dataset
  * `table` is the name of the existing BigQuery dataset table
  * `batch_size` is the size of the insert batch, every n number of messages the service will insert batch into BigQuery. Should be lesser than the maximum size of [BigQuery batch insert limits](https://cloud.google.com/bigquery/quotas#load_jobs)
  * `ignore_unknowns` indicates whether the service should error when there are fields in your JSON message on PubSun that are not represented in BigQuery table
* `max_duration` is the maximum amount of time the service should execute. The service will exit after the specified number of seconds whether there are more message or not. Should not be greater than the service `--timeout` or maximum Cloud Run service execution time (15 min)

You can create multiple jobs each with their own schedules and configuration. To see a sample of the schedule that will execute every 30 min see [bin/schedule](./bin/schedule), then to execute it with your own job run:

Note, cloud run service URLs are not predictable, you can capture your service URL using this command:

```shell
gcloud beta run services describe $SERVICE_NAME \
    --region $SERVICE_REGION \
    --format="value(status.domain)"
```


## Metrics

Following custom metrics are being recorded in Stackdriver for each service invocation.

* `custom/metric/invocation` - number of times the pump service was invoked
* `custom/metric/message` - number of messages processed for each job invocation
* `custom/metric/duration` - total duration (in seconds) of each job invocation

## Demo

To quickly evaluate this service you can use [PubSub Event Maker](https://github.com/mchmarny/pubsub-event-maker) with the following configuration

### Setup PubSub

Create PubSub topic named `pump`

```shell
gcloud pubsub topics create pump
```

Then create a PubSub subscription named `pump-sub`

```shell
gcloud pubsub subscriptions create pump-sub \
    --topic pump \
    --ack-deadline 600 \
    --message-retention-duration 1d
```

### Setup BigQuery

Create a BiqQuery dataset named `pump`

```shell
bq mk pump
```

Then create a BigQuery table named `events` with a simple schema

```shell
bq mk --schema source_id:string,event_id:string,event_ts:timestamp,load_1:numeric \
      -t "pump.events"
```

### Send Data

Clone the [PubSub Event Maker](https://github.com/mchmarny/pubsub-event-maker) repo or download the latest release for your OS and run it to push data to your newly created PubSub topic

```shell
./eventmaker --topic=pump --sources=3 --metric=demo --freq=0.5s
```

This will mock `demo` metric messages from `3` different sources at `0.5` second frequency. The content of the submitted events will be printed out in the console like this

```shell
[EVENT-MAKER] Publishing: {"source_id":"device-2","event_id":"eid-eb3ac691348f","event_ts":"2019-11-03T14:50:50.485636Z","label":"demo","mem_used":90.8284,"cpu_used":58.6633,"load_1":5.06,"load_5":11.92,"load_15":34.36,"random_metric":20.3186}
```

### Execute Scheduler

To execute service now you can manually trigger the created schedule. If everything goes well your BigQuery table should have some messages.


## Building Image

If you prefer to build your own image you can submit a job to the Cloud Build service using the included [Dockerfile](./Dockerfile) and results in versioned, non-root container image URI which will be used to deploy your service to Cloud Run.

```shell
bin/image
```

## Cleanup

To cleanup all resources created by this sample execute

```shell
bin/cleanup
```

## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.


