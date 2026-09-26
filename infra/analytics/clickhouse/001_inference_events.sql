create table if not exists inference_events (
  event_id String, provider LowCardinality(String), model LowCardinality(String),
  status LowCardinality(String), latency_ms UInt64, input_tokens UInt32, output_tokens UInt32,
  completed_at DateTime64(3, 'UTC')
) engine = MergeTree order by (completed_at, provider, model);

create table if not exists kafka_inference_raw (payload String)
engine = Kafka settings kafka_broker_list = 'kafka:9092', kafka_topic_list = 'inference.events',
  kafka_group_name = 'clickhouse-ollive', kafka_format = 'RawBLOB';

create materialized view if not exists kafka_inference_mv to inference_events as select
  JSONExtractString(payload, 'id') as event_id,
  JSONExtractString(payload, 'provider') as provider,
  JSONExtractString(payload, 'model') as model,
  JSONExtractString(payload, 'status') as status,
  toUInt64OrZero(JSONExtractString(payload, 'latency_ms')) as latency_ms,
  toUInt32OrZero(JSONExtractString(payload, 'input_tokens')) as input_tokens,
  toUInt32OrZero(JSONExtractString(payload, 'output_tokens')) as output_tokens,
  parseDateTime64BestEffort(JSONExtractString(payload, 'completed_at'), 3) as completed_at
from kafka_inference_raw;
