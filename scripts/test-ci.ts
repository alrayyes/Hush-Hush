// Tests the pipeline's own wiring: which CI jobs run for a change, whether the
// pre-push hook runs the same ones, and the structural rules a stale list
// breaks quietly. Run with `bun run test:ci`; CI and the pre-push hook both do.
//
// rules/ci.md: "A job's real inputs are wider than its name ... give the
// filter a test with one case per path". A filter written from a job's name
// skips a job that could fail, and nothing else notices until it does.
//
// Nothing here is a copy of the workflow: which filters gate which job is read
// from each job's `if:`, and the filters from the `changes` job itself, so
// this can't drift from what it checks.

import { readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import picomatch from 'picomatch';

type Workflow = {
  on?: { pull_request?: { paths?: string[] } };
  jobs: Record<
    string,
    {
      if?: string;
      needs?: string | string[];
      steps?: { id?: string; with?: Record<string, string> }[];
    }
  >;
};

const failures: string[] = [];
const fail = (message: string) => failures.push(message);

const read = (path: string) => Bun.file(path).text();
const yaml = async (path: string) => Bun.YAML.parse(await read(path)) as Workflow;

const ci = await yaml('.github/workflows/ci.yml');
const prose = await yaml('.github/workflows/prose.yml');

// dorny/paths-filter matches with picomatch ^2.3.1 and `dot: true`, one pattern
// at a time, OR-ed together. Use the same library and options so a case here
// means what it means in CI.
const filterStep = ci.jobs.changes.steps?.find((s) => s.id === 'filter');
if (!filterStep?.with?.filters) throw new Error("ci.yml's `changes` job has no filters to test");
const filters = Bun.YAML.parse(filterStep.with.filters) as Record<string, string[]>;
const matchesAny = (patterns: string[], path: string) =>
  patterns.some((pattern) => picomatch(pattern, { dot: true })(path));
const filterMatches = (name: string, path: string) => matchesAny(filters[name] ?? [], path);

// A job that gates on `needs.changes.outputs.<filter> == 'true'` runs when any
// of its filters matches. Read that from the workflow instead of restating it.
const gatedBy = (expression: string) =>
  [...expression.matchAll(/needs\.changes\.outputs\.([a-z-]+) == 'true'/g)].map((m) => m[1]);
const filteredJobs: Record<string, string[]> = {};
for (const [name, job] of Object.entries(ci.jobs)) {
  const used = job.if ? gatedBy(job.if) : [];
  if (used.length > 0) filteredJobs[name] = used;
}
const jobsThatRun = (path: string) =>
  Object.entries(filteredJobs)
    .filter(([, used]) => used.some((filter) => filterMatches(filter, path)))
    .map(([name]) => name)
    .sort();

// ---- 1. One case per path: which CI jobs run -------------------------------

const GO = ['build', 'gomod', 'govulncheck', 'lint', 'pact', 'test'];
const cases: [path: string, expected: string[]][] = [
  ['internal/api/create.go', [...GO, 'container-integration', 'e2e', 'lighthouse']],
  ['main.go', [...GO, 'container-integration']],
  ['go.mod', [...GO, 'container-integration', 'ci-config']],
  ['go.sum', [...GO, 'container-integration']],
  ['.golangci.yml', [...GO, 'container-integration', 'prose']],
  ['api/openapi.yaml', [...GO, 'container-integration', 'e2e', 'lighthouse', 'web', 'api', 'prose']],
  ['cmd/hush-hush/web/src/app.css', ['web', 'e2e', 'lighthouse', 'container-integration']],
  ['cmd/hush-hush/web/build/index.html', [...GO, 'web', 'e2e', 'lighthouse', 'container-integration']],
  ['pacts/hush-hush-cli-hush-hush-server.json', [...GO, 'container-integration']],
  ['integration/container_test.go', [...GO, 'container-integration']],
  ['README.md', ['prose']],
  ['CONTRIBUTING.md', ['prose']],
  ['CHANGELOG.md', ['web', 'prose']],
  ['LICENSE', ['web']],
  ['Dockerfile', ['dockerfile', 'container-integration', 'ci-config']],
  ['Dockerfile.release', ['dockerfile', 'goreleaser', 'container-integration', 'ci-config']],
  // .dockerignore decides what the image build sees, so the integration test that
  // builds it has to run too.
  ['.dockerignore', ['dockerfile', 'container-integration']],
  ['.hadolint.yaml', ['dockerfile', 'container-integration', 'prose']],
  ['.goreleaser.yml', ['goreleaser', 'prose']],
  ['compose.yaml', ['container-integration', 'prose']],
  ['package.json', ['web', 'prose', 'api', 'package-json', 'bun-audit', 'ci-config']],
  ['bun.lock', ['web', 'prose', 'api', 'bun-audit', 'ci-config']],
  ['redocly.yaml', ['api', 'prose']],
  ['.spectral.yaml', ['api', 'prose']],
  ['docs/api/index.html', ['api']],
  ['.markdownlint-cli2.yaml', ['prose']],
  ['.vale.ini', ['prose']],
  ['styles/config/vocabularies/House/accept.txt', ['prose']],
  // prettier reads .editorconfig, so the prose job's result can change with it.
  ['.editorconfig', ['prose']],
  // ltex runs in prose.yml, not here, so ci.yml has nothing to run for it.
  ['.ltex.json', []],
  // Editing the pipeline's own config runs what it configures.
  ['.github/workflows/ci.yml', [...GO, 'container-integration', 'e2e', 'lighthouse', 'web', 'api', 'dockerfile', 'goreleaser', 'package-json', 'bun-audit', 'prose', 'ci-config']],
  // Hook config is yaml, so prettier covers it, and nothing else in ci.yml reads it.
  ['lefthook.yml', ['prose', 'ci-config']],
  ['scripts/test-ci.ts', ['ci-config']],
];

for (const [path, expected] of cases) {
  const got = jobsThatRun(path);
  const want = [...expected].sort();
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    const missing = want.filter((j) => !got.includes(j));
    const extra = got.filter((j) => !want.includes(j));
    fail(
      `ci.yml filters: ${path}` +
        (missing.length ? `\n    should run but doesn't: ${missing.join(', ')}` : '') +
        (extra.length ? `\n    runs but shouldn't: ${extra.join(', ')}` : ''),
    );
  }
}

// ---- 2. The pre-push hook runs the same jobs --------------------------------

// Which ci.yml job each hook job is the local half of. vale and mechanics run
// in prose.yml, not ci.yml; the ci.yml prose filter is the one they mirror.
const hookToCi: Record<string, string> = {
  'golangci-lint fmt': 'lint',
  'golangci-lint run': 'lint',
  test: 'test',
  gomod: 'gomod',
  'web-check': 'web',
  'web-lint': 'web',
  'web-lint-tailwind': 'web',
  'web-api-types': 'web',
  prettier: 'prose',
  markdownlint: 'prose',
  vale: 'prose',
  mechanics: 'prose',
  redocly: 'api',
  spectral: 'api',
  'sort-package-json': 'package-json',
  'container-integration': 'container-integration',
  'ci-config': 'ci-config',
};

// The one deliberate difference: a change to lefthook.yml itself runs every
// hook job locally, as the pre-push globs say, though CI has nothing to run.
const hook = Bun.YAML.parse(await read('lefthook.yml')) as {
  'pre-push': { jobs: { name: string }[] };
};
const hookJobs = hook['pre-push'].jobs.map((j) => j.name);
for (const name of hookJobs) {
  if (!(name in hookToCi)) {
    fail(`lefthook.yml: pre-push job "${name}" isn't mapped to a ci.yml job in scripts/test-ci.ts`);
  }
}

// Run the real lefthook with each job's command swapped for an echo, one
// pretend push per path, so the hook's own glob matcher decides.
const echoConfig = Bun.YAML.parse(await read('lefthook.yml')) as Record<string, unknown>;
delete echoConfig.output;
const echoJobs = (echoConfig['pre-push'] as { jobs: { name: string; run?: string; root?: string }[] }).jobs;
for (const job of echoJobs) {
  job.run = `echo RAN:${job.name}`;
  delete job.root;
}
const echoPath = join(tmpdir(), `lefthook-echo-${process.pid}.yml`);
await Bun.write(echoPath, Bun.YAML.stringify({ 'pre-push': echoConfig['pre-push'] }));

const hookJobsThatRun = (path: string) => {
  const result = Bun.spawnSync(['bunx', 'lefthook', 'run', 'pre-push', '--file', path], {
    env: { ...process.env, LEFTHOOK_CONFIG: echoPath },
  });
  const out = new TextDecoder().decode(result.stdout) + new TextDecoder().decode(result.stderr);
  return [...out.matchAll(/RAN:([^\n\r]+)/g)].map((m) => m[1].trim()).sort();
};

for (const [path] of cases) {
  const ciRuns = new Set(jobsThatRun(path));
  const expected =
    path === 'lefthook.yml'
      ? [...hookJobs]
      : hookJobs.filter((name) => ciRuns.has(hookToCi[name] ?? name));
  const got = hookJobsThatRun(path);
  const want = [...new Set(expected)].sort();
  if (JSON.stringify(got) !== JSON.stringify(want)) {
    const missing = want.filter((j) => !got.includes(j));
    const extra = got.filter((j) => !want.includes(j));
    fail(
      `pre-push hook vs ci.yml: ${path}` +
        (missing.length ? `\n    CI would run these, the hook doesn't: ${missing.join(', ')}` : '') +
        (extra.length ? `\n    the hook runs these, CI wouldn't: ${extra.join(', ')}` : ''),
    );
  }
}

// ---- 3. prose.yml's own filter covers the scripts it runs -------------------

// prose.yml is gated with workflow-level `paths:`, so a script it runs and the
// filter doesn't list can change without the check running.
const proseFilter = prose.on?.pull_request?.paths ?? [];
for (const input of [
  'scripts/lint-prose.sh',
  'scripts/lint-mechanics.sh',
  'scripts/vale.sh',
  '.github/workflows/prose.yml',
  '.vale.ini',
  '.ltex.json',
  'styles/House/anything.yml',
  'README.md',
]) {
  if (!matchesAny(proseFilter, input)) fail(`prose.yml paths: doesn't list ${input}, which its jobs read`);
}

// ---- 4. Lists that go stale without anyone noticing -------------------------

const allJobs = Object.keys(ci.jobs);
const asList = (needs: string | string[] | undefined) => (typeof needs === 'string' ? [needs] : (needs ?? []));
const sameSet = (a: string[], b: string[]) => JSON.stringify([...a].sort()) === JSON.stringify([...b].sort());

// A deploy waits for every other job (rules/ci.md). all-green can't be one of
// them, since it waits on jobs the deploy also waits on.
const pagesWants = allJobs.filter((j) => !['pages', 'all-green'].includes(j));
if (!sameSet(asList(ci.jobs.pages?.needs), pagesWants)) {
  fail(`ci.yml: pages.needs should be every other job: ${pagesWants.join(', ')}`);
}

// The aggregator (when there is one) waits on every merge-gating job and may
// only skip the filtered ones. `pages` is the deploy and `lighthouse` warns.
const allGreen = ci.jobs['all-green'];
if (allGreen) {
  const wants = allJobs.filter((j) => !['pages', 'lighthouse', 'all-green'].includes(j));
  if (!sameSet(asList(allGreen.needs), wants)) fail(`ci.yml: all-green.needs should be: ${wants.join(', ')}`);
  const skips = (allGreen.steps?.[0]?.with?.['allowed-skips'] ?? '').split(',').map((s) => s.trim()).filter(Boolean);
  const mayskip = wants.filter((j) => j !== 'changes');
  if (!sameSet(skips, mayskip)) fail(`ci.yml: all-green's allowed-skips should be every job but changes`);
}

// One Go toolchain image everywhere: the Dockerfile, the hook and every
// workflow. Dependabot bumps the Dockerfile only, so the rest drift.
const imageRef = /golang:[0-9][0-9.]*-bookworm@sha256:[0-9a-f]{64}/g;
const imageFiles = [
  'Dockerfile',
  'Dockerfile.release',
  'lefthook.yml',
  ...readdirSync('.github/workflows').filter((f) => f.endsWith('.yml')).map((f) => `.github/workflows/${f}`),
];
const imageUses = new Map<string, string[]>();
for (const file of imageFiles) {
  for (const ref of new Set((await read(file).catch(() => '')).match(imageRef) ?? [])) {
    imageUses.set(ref, [...(imageUses.get(ref) ?? []), file]);
  }
}
if (imageUses.size > 1) {
  fail(
    'golang images disagree:\n' +
      [...imageUses].map(([ref, files]) => `    ${ref.slice(0, 40)}...  ${files.join(', ')}`).join('\n'),
  );
}

// pact-go's native library has to match the Go module.
const goMod = await read('go.mod');
const moduleVersion = goMod.match(/pact-foundation\/pact-go\/v2 (v[0-9.]+)/)?.[1];
const installed = (await read('.github/workflows/ci.yml')).match(/pact-go\/v2@(v[0-9.]+)/)?.[1];
if (moduleVersion && installed && moduleVersion !== installed) {
  fail(`ci.yml installs pact-go ${installed} but go.mod has ${moduleVersion}`);
}

// ---- result -----------------------------------------------------------------

if (failures.length > 0) {
  console.error(`${failures.length} problem${failures.length === 1 ? '' : 's'} in the pipeline wiring:\n`);
  for (const message of failures) console.error(`- ${message}`);
  process.exit(1);
}
console.log(`ok: ${cases.length} paths, ${hookJobs.length} hook jobs, structure checks`);
