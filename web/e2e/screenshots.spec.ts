import { test, expect } from '@playwright/test';
import { setupMocks, injectAuth } from './fixtures/api-mocks';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const screenshotDir = path.join(__dirname, '../../assets/screenshots');

test.beforeEach(async ({ page }) => {
  await injectAuth(page);
  await setupMocks(page);
});

function screenshotPath(name: string) {
  return path.join(screenshotDir, `${name}.png`);
}

test('dashboard', async ({ page }) => {
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText('api.github.com')).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('dashboard'), fullPage: true });
});

test('connections', async ({ page }) => {
  await page.goto('/connections');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('connections'), fullPage: true });
});

test('rules', async ({ page }) => {
  await page.goto('/rules');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('rules'), fullPage: true });
});

test('nodes', async ({ page }) => {
  await page.goto('/nodes');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('nodes'), fullPage: true });
});

test('stats', async ({ page }) => {
  await page.goto('/stats/hosts');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('stats'), fullPage: true });
});

test('dns', async ({ page }) => {
  await page.goto('/dns');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('dns'), fullPage: true });
});

test('blocklists', async ({ page }) => {
  await page.goto('/blocklists');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('blocklists'), fullPage: true });
});

test('templates', async ({ page }) => {
  await page.goto('/templates');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('templates'), fullPage: true });
});

test('alerts', async ({ page }) => {
  await page.goto('/alerts');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText('312 connections denied in the last hour on gateway-01')).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('alerts'), fullPage: true });
});

test('settings', async ({ page }) => {
  await page.goto('/settings');
  await page.waitForLoadState('networkidle');
  await expect(page.getByText('This page now shows build information only.')).toBeVisible();
  await page.waitForTimeout(500);
  await page.screenshot({ path: screenshotPath('settings'), fullPage: true });
});
