require 'spec_helper'
require 'webmock/rspec'
require_relative '../plugins/loggregator'

RSpec.describe 'Loggregator Fluentd' do
  context 'LoggregatorOutput' do
    it 'writes logs to the grpc stub' do
      now = Time.now
      expectedBatch = Loggregator::V2::EnvelopeBatch.new
      env = Loggregator::V2::Envelope.new
      log = Loggregator::V2::Log.new
      log.type = :ERR
      log.payload = 'test_payload'
      env.log = log
      env.timestamp = (now.to_f * (10**9)).to_i
      env.source_id = 'test_owner'
      env.instance_id = 'test_pod_id'
      env.tags['source_type'] = 'APP/PROC/WEB'
      env.tags['namespace'] = 'test_namespace'
      env.tags['container'] = 'test_container'
      env.tags['cluster'] = 'test_host'
      env.tags['pod_name'] = 'test_pod_name'
      expectedBatch.batch << env

      output = Fluent::Plugin::LoggregatorOutput.new

      grpc = double('gRPC stub')
      output.instance_variable_set(:@stub, grpc)
      expect(grpc).to receive(:send).with(expectedBatch)

      output.emit('tag', [[now, {
                    'log' => 'test_payload',
                    'stream' => 'stderr',
                    'kubernetes' => {
                      'owner' => 'test_owner',
                      'pod_id' => 'test_pod_id',
                      'pod_name' => 'test_pod_name',
                      'namespace_name' => 'test_namespace',
                      'container_name' => 'test_container',
                      'host' => 'test_host',
                      'labels' => {
                          'guid' => 'test_owner',
                      },
                    }
                  }]], '')
    end

    context 'when it is a statefulset pod'
      it 'extracts the instance id from the pod name' do
        now = Time.now
        expectedBatch = Loggregator::V2::EnvelopeBatch.new
        env = Loggregator::V2::Envelope.new
        log = Loggregator::V2::Log.new
        log.type = :ERR
        log.payload = 'test_payload'
        env.log = log
        env.timestamp = (now.to_f * (10**9)).to_i
        env.source_id = 'test_owner'
        env.instance_id = '44'
        env.tags['source_type'] = 'APP/PROC/WEB'
        env.tags['namespace'] = 'test_namespace'
        env.tags['container'] = 'test_container'
        env.tags['cluster'] = 'test_host'
        env.tags['pod_name'] = 'test_pod_name-44'
        expectedBatch.batch << env

        output = Fluent::Plugin::LoggregatorOutput.new

        grpc = double('gRPC stub')
        output.instance_variable_set(:@stub, grpc)
        expect(grpc).to receive(:send).with(expectedBatch)

        output.emit('tag', [[now, {
                      'log' => 'test_payload',
                      'stream' => 'stderr',
                      'kubernetes' => {
                        'owner' => 'test_owner',
                        'pod_id' => 'test-pod-id',
                        'pod_name' => 'test_pod_name-44',
                        'namespace_name' => 'test_namespace',
                        'container_name' => 'test_container',
                        'host' => 'test_host',
                        'labels' => {
                            'guid' => 'test_owner',
                        },
                      }
                    }]], '')
      end

      context 'when it is a staging job'
        it 'extracts the source_type of a staging job from the k8s labels' do
          now = Time.now
          expectedBatch = Loggregator::V2::EnvelopeBatch.new
          env = Loggregator::V2::Envelope.new
          log = Loggregator::V2::Log.new
          log.type = :ERR
          log.payload = 'test_payload'
          env.log = log
          env.timestamp = (now.to_f * (10**9)).to_i
          env.instance_id = '44'
          env.tags['source_type'] = 'STG'
          env.tags['namespace'] = 'test_namespace'
          env.tags['container'] = 'test_container'
          env.tags['cluster'] = 'test_host'
          env.tags['pod_name'] = 'test_pod_name-44'
          expectedBatch.batch << env

          output = Fluent::Plugin::LoggregatorOutput.new

          grpc = double('gRPC stub')
          output.instance_variable_set(:@stub, grpc)
          expect(grpc).to receive(:send).with(expectedBatch)

          output.emit('tag', [[now, {
              'log' => 'test_payload',
              'stream' => 'stderr',
              'kubernetes' => {
                  'owner' => 'test_owner',
                  'pod_id' => 'test-pod-id',
                  'pod_name' => 'test_pod_name-44',
                  'namespace_name' => 'test_namespace',
                  'container_name' => 'test_container',
                  'host' => 'test_host',
                  'labels' => {
                      'source_type' => 'STG',
                  },
              }
          }]], '')
      end

    end

  context 'SourceIDFilter' do
    it 'asks the kubernetes client for owner information' do
      f = Fluent::Plugin::SourceIDFilter.new
      kclient = double('kubernetes client stub')
      f.instance_variable_set(:@client, kclient)
      f.instance_variable_set(:@cache, {})
      f.instance_variable_set(:@namespace, "test_namespace_name")
      expect(kclient).to receive(:get_pod).with(
        'test_pod_name',
        'test_namespace_name'
      ) {
        {
          'metadata' => {
            'ownerReferences' => [
              {
                'kind' => 'ReplicaSet',
                'name' => 'test_replicaset_name'
              }
            ]
          }
        }
      }
      expect(kclient).to receive(:get_replicaset).with(
        'test_replicaset_name',
        'test_namespace_name'
      )

      record = {
        'kubernetes' => {
          'pod_name' => 'test_pod_name',
          'namespace_name' => 'test_namespace_name'
        }
      }
      record = f.filter(nil, nil, record)
      expect(record['kubernetes']['owner']).to eq('test_replicaset_name')
    end

    it 'caches results' do
      f = Fluent::Plugin::SourceIDFilter.new
      kclient = double('kubernetes client stub')
      f.instance_variable_set(:@client, kclient)
      f.instance_variable_set(:@cache, {})
      f.instance_variable_set(:@namespace, "test_namespace_name")
      expect(kclient).to receive(:get_pod).with(
        'test_pod_name',
        'test_namespace_name'
      )
      record = {
        'kubernetes' => {
          'pod_name' => 'test_pod_name',
          'namespace_name' => 'test_namespace_name'
        }
      }
      record = f.filter(nil, nil, record)
      expect(record['kubernetes']['owner']).to eq('test_pod_name')
      record = f.filter(nil, nil, record)
      expect(record['kubernetes']['owner']).to eq('test_pod_name')
    end
  end
end

RSpec.describe 'Kubernetes Client' do
  it 'can get a pod' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/api/v1/namespaces/test_ns/pods/test_pod')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_pod('test_pod', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a replicationcontroller' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/api/v1/namespaces/test_ns/replicationcontrollers/test_rc')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_replicationcontroller('test_rc', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a replicaset' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/apis/apps/v1/namespaces/test_ns/replicasets/test_rs')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_replicaset('test_rs', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a deployment' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/apis/apps/v1/namespaces/test_ns/deployments/test_deploy')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_deployment('test_deploy', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a daemonset' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/apis/apps/v1/namespaces/test_ns/daemonsets/test_ds')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_daemonset('test_ds', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a statefulset' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/apis/apps/v1/namespaces/test_ns/statefulsets/test_sts')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_statefulset('test_sts', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a job' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/apis/batch/v1/namespaces/test_ns/jobs/test_job')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_job('test_job', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end

  it 'can get a cronjob' do
    client = KubernetesClient.new(token: 'token')
    stub_request(:get, 'https://kubernetes.default.svc.cluster.local/apis/batch/v1beta1/namespaces/test_ns/cronjobs/test_cronjob')
      .with(headers: { 'Authorization' => 'Bearer token' })
      .to_return(body: '{"foo":"bar"}')
    response = client.get_cronjob('test_cronjob', 'test_ns')
    expect(response).to eq('foo' => 'bar')
  end
end
