import assert from 'node:assert/strict';
import { test } from 'node:test';

import { normalizeName } from '../src/lib/hello';

test('名字去掉首尾空白，空输入不发请求', () => {
  assert.equal(normalizeName('  world '), 'world');
  assert.equal(normalizeName('世界'), '世界');
  for (const empty of [null, undefined, '', '   ']) {
    assert.equal(normalizeName(empty), null, String(empty));
  }
});
