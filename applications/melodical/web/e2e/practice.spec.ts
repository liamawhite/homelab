import { expect, test, type Page } from '@playwright/test';
import type { Question } from '../src/types';
async function chooseScale(page: Page) {
  for (const midi of [48, 50, 52, 53, 55, 57, 59]) {
    await page.locator(`[data-midi="${midi}"]`).click();
  }
  await page.getByRole('button', { name: 'Check answer', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'That’s it. Nicely connected.' }),
  ).toBeVisible();
}
test('phone-friendly practice, resume, feedback, and shared progress', async ({
  page,
  context,
}) => {
  await page.goto('/');
  await expect(
    page.getByRole('button', { name: 'Start practicing', exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole('heading', { name: 'Make music make sense.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Piano', exact: true }).click();
  await page
    .getByRole('combobox', { name: 'Focus', exact: true })
    .selectOption('keys');
  await page.getByRole('button', { name: 'Quick round', exact: false }).click();
  await page
    .getByRole('combobox', { name: 'Round length', exact: true })
    .selectOption('5');
  await page
    .getByRole('button', { name: 'Key & difficulty', exact: false })
    .click();
  await page
    .getByRole('combobox', { name: 'Musical key', exact: true })
    .selectOption('C');
  await page
    .getByRole('combobox', { name: 'Difficulty', exact: true })
    .selectOption('1');
  await page
    .getByRole('button', { name: 'Start practicing', exact: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Find the notes of the C major scale' }),
  ).toBeVisible();
  await chooseScale(page);
  await page.getByRole('button', { name: 'Next question' }).click();
  // An incorrect answer teaches and returns later in the same session.
  await page.locator('[data-midi="49"]').click();
  await page.getByRole('button', { name: 'Check answer', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'A good moment to connect the dots.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Pause practice' }).click();
  await expect(page.getByText('Your practice is right here.')).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole('button', { name: 'Resume', exact: false }),
  ).toBeEnabled();
  await page.getByRole('button', { name: 'Resume', exact: false }).click();
  await expect(
    page.getByRole('heading', { name: 'A good moment to connect the dots.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Next question' }).click();
  for (let i = 0; i < 3; i++) {
    await chooseScale(page);
    await page
      .getByRole('button', { name: i === 2 ? 'Finish round' : 'Next question' })
      .click();
  }
  await expect(
    page.getByRole('heading', { name: 'Practice, made progress.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Progress', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'See what’s clicking.' }),
  ).toBeVisible();
  await expect(page.getByText('Your practice map.')).toBeVisible();
  await expect(
    page.locator('.skill-card').filter({ hasText: 'Notes in a key' }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  const second = await context.browser()!.newContext();
  const other = await second.newPage();
  await other.goto('/');
  await other.getByRole('button', { name: 'Progress', exact: true }).click();
  await expect(
    other.locator('.skill-card').filter({ hasText: 'Notes in a key' }),
  ).toBeVisible();
  await second.close();
});
test('guitar region, hints, theme, and field notes', async ({ page }) => {
  await page.goto('/');
  await expect(
    page.getByRole('button', { name: 'Start practicing', exact: true }),
  ).toBeEnabled();
  await page.getByRole('button', { name: 'Guitar', exact: true }).click();
  await page
    .getByRole('combobox', { name: 'Focus', exact: true })
    .selectOption('notes');
  await page
    .getByRole('button', { name: 'Free practice', exact: false })
    .click();
  await page
    .getByRole('button', { name: 'Start practicing', exact: true })
    .click();
  await expect(page.locator('.fretboard')).toBeVisible();
  await page
    .getByRole('button', { name: 'Show note labels', exact: false })
    .click();
  const prompt = await page.locator('.question-heading h1').innerText();
  const name = prompt.replace('Find ', '');
  const enabled = page
    .locator('.fret-cell button:not(:disabled)')
    .filter({ hasText: name });
  await enabled.first().click();
  await expect(
    page.getByRole('heading', { name: 'You found it.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Finish', exact: true }).click();
  await page.getByRole('button', { name: 'Field notes', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Little field notes.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Theme:', exact: false }).click();
  expect(await page.locator('html').getAttribute('data-theme')).toBeTruthy();
  await page.getByRole('button', { name: 'Practice', exact: true }).click();
  await page.screenshot({
    path: `../.tmp/home-${test.info().project.name}.png`,
    fullPage: true,
  });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
});
test('inversions accept a correct chord spanning phone octaves', async ({
  page,
}) => {
  await page.goto('/');
  await expect(
    page.getByRole('button', { name: 'Start practicing', exact: true }),
  ).toBeEnabled();
  await page.getByRole('button', { name: 'Piano', exact: true }).click();
  await page
    .getByRole('combobox', { name: 'Focus', exact: true })
    .selectOption('inversions');
  await page
    .getByRole('button', { name: 'Key & difficulty', exact: false })
    .click();
  await page
    .getByRole('combobox', { name: 'Musical key', exact: true })
    .selectOption('B');
  await page
    .getByRole('combobox', { name: 'Difficulty', exact: true })
    .selectOption('1');
  let q: Question | null = null;
  for (let i = 0; i < 20; i++) {
    const res = await page.request.post('/api/questions', {
      data: {
        instrument: 'piano',
        topic: 'inversions',
        key: 'B',
        difficulty: '1',
      },
    });
    q = await res.json();
    if (q.kind === 'chord') break;
  }
  if (!q || q.kind !== 'chord')
    throw new Error('No chord construction question generated');
  expect(q.kind).toBe('chord');
  await page.route('**/api/questions', async (route) => {
    await route.fulfill({ json: q });
    await page.unroute('**/api/questions');
  });
  await page
    .getByRole('button', { name: 'Start practicing', exact: true })
    .click();
  await expect(page.locator('.question-heading h1')).toHaveText(q.prompt);
  const third = q.prompt.includes('minor') ? 3 : 4;
  const first = q.prompt.includes('first inversion');
  const notes = first ? [59 + third, 66, 71] : [54, 59, 59 + third];
  let octave = 0;
  for (const midi of notes) {
    const target = midi >= 60 ? 1 : 0;
    if (
      (await page.locator('.octave-controls').isVisible()) &&
      target !== octave
    ) {
      await page
        .getByRole('button', {
          name: target ? 'Next octave' : 'Previous octave',
          exact: true,
        })
        .click();
      octave = target;
    }
    await page.locator(`[data-midi="${midi}"]`).click();
  }
  await page.getByRole('button', { name: 'Check answer', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'That’s it. Nicely connected.' }),
  ).toBeVisible();
  await page.screenshot({
    path: `../.tmp/inversion-${test.info().project.name}.png`,
    fullPage: true,
  });
});
