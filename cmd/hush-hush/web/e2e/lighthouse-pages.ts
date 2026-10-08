// The pages e2e/lighthouse.ts audits, one per route of the app. The spec beside
// it fails when a route is added to src/routes and left out here.

// The secret the signed-in run seeds, so its detail page has something on it.
export const SEEDED_SLUG = 'mattermost_deploy_webhook';

export interface AuditedPage {
	/** The SvelteKit route, as the directory under src/routes names it. */
	route: string;
	/** The report's file name: <report>.report.html and <report>.report.json. */
	report: string;
	/** What Lighthouse is pointed at, and the path it has to land on. */
	path: string;
	/** Whether the page needs the signed-in session. */
	session: boolean;
}

// Public pages first, then the ones behind the session: the run registers a
// passkey between the two groups, and every later audit keeps that session.
export const AUDITED_PAGES: AuditedPage[] = [
	{ route: '/login', report: 'login', path: '/login', session: false },
	{
		route: '/changelog',
		report: 'changelog',
		path: '/changelog',
		session: false,
	},
	{
		route: '/disclaimer',
		report: 'disclaimer',
		path: '/disclaimer',
		session: false,
	},
	{ route: '/privacy', report: 'privacy', path: '/privacy', session: false },
	{ route: '/license', report: 'license', path: '/license', session: false },
	{ route: '/', report: 'secrets-overview', path: '/', session: true },
	{
		route: '/secrets/[slug]',
		report: 'secret-detail',
		path: `/secrets/${SEEDED_SLUG}`,
		session: true,
	},
	{
		route: '/consumers',
		report: 'consumers',
		path: '/consumers',
		session: true,
	},
	{
		route: '/audit-log',
		report: 'audit-log',
		path: '/audit-log',
		session: true,
	},
	{ route: '/settings', report: 'settings', path: '/settings', session: true },
];
