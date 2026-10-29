import { expect, test } from '@playwright/test';

test.describe('B 端后台 · Hello', () => {
  test('首页默认进入 Hello', async ({ page }) => {
    await page.goto('');
    await expect(page).toHaveURL(/\/hello/);
    await expect(page.locator('body')).toContainText('Beidou Layout');
  });

  test('输入名字打招呼，显示问候并出现在最近记录里', async ({ page }) => {
    const name = `e2e-${Date.now()}`;
    await page.goto('hello');
    await page.getByPlaceholder('输入名字').fill(name);
    await page.getByRole('button', { name: '打招呼' }).click();
    await expect(page.getByText(`Hello, ${name}!`).first()).toBeVisible();
  });
});

test.describe('B 端后台 · 存储', () => {
  test('存储供应商与上传示例页面可打开', async ({ page }) => {
    await page.goto('hello');
    await page.getByRole('button', { name: '存储供应商' }).click();
    await expect(page).toHaveURL(/\/storage\/providers$/);
    await expect(page.getByRole('button', { name: '新建供应商' })).toBeVisible();
    await page.getByRole('button', { name: '上传示例' }).click();
    await expect(page).toHaveURL(/\/storage\/upload$/);
    await expect(page.getByRole('button', { name: '选择文件上传' })).toBeVisible();
  });
});
