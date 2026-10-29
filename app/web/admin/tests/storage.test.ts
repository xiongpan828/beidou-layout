import assert from 'node:assert/strict';
import { test } from 'node:test';

import { buildProviderConfig, parseProviderConfig } from '../src/lib/storage';

test('供应商配置只带该类型的字段，空串不带、布尔值总带', () => {
  const form = {
    endpoint: ' 127.0.0.1:9000 ',
    access_key_id: 'ak',
    secret_access_key: '',
    bucket: 'b',
    region: '',
    secure: undefined,
    use_path_style: true,
  };
  assert.deepEqual(JSON.parse(buildProviderConfig('minio', form)), {
    endpoint: '127.0.0.1:9000',
    access_key_id: 'ak',
    bucket: 'b',
    secure: false,
  });
  assert.deepEqual(JSON.parse(buildProviderConfig('s3', { ...form, endpoint: '', region: 'us-east-1' })), {
    access_key_id: 'ak',
    bucket: 'b',
    region: 'us-east-1',
    use_path_style: true,
  });
});

test('解析配置时清空密钥，非法 JSON 返回空表单', () => {
  assert.deepEqual(parseProviderConfig('{"bucket":"b","secret_access_key":"x"}'), { bucket: 'b', secret_access_key: '' });
  assert.deepEqual(parseProviderConfig('not json'), {});
  assert.deepEqual(parseProviderConfig(undefined), { secret_access_key: '' });
});
