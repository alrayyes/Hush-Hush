import type { ObjectMetadata } from './api';
import { actorName } from './attribution';

// The secrets overview's filters that live in the URL, so a filtered view
// survives a reload and can be shared. Text and tag stay local state.
// usedBy asks the API (GET /objects?used_by=, one call per consumer, any of
// them matches); the rest filter the loaded list, since GET /objects has no
// query for them and returns every object.
export type ObjectFilters = {
	usedBy: string[];
	createdBy?: string;
	createdSince?: string;
	updatedSince?: string;
};

const DAY = /^\d{4}-\d{2}-\d{2}$/;

export function parseObjectFilters(params: URLSearchParams): ObjectFilters {
	const filters: ObjectFilters = {
		usedBy: params.getAll('used_by').filter((name) => name !== ''),
	};
	const createdBy = params.get('created_by');
	const createdSince = params.get('created_since');
	const updatedSince = params.get('updated_since');

	if (createdBy) filters.createdBy = createdBy;
	if (createdSince && DAY.test(createdSince)) {
		filters.createdSince = createdSince;
	}
	if (updatedSince && DAY.test(updatedSince)) {
		filters.updatedSince = updatedSince;
	}

	return filters;
}

// parsePage reads ?page= from the URL. Anything that isn't a whole number from
// 1 up is the first page; a page past the end is clamped where it is shown.
export function parsePage(params: URLSearchParams): number {
	const page = Number(params.get('page'));

	return Number.isInteger(page) && page >= 1 ? page : 1;
}

// objectFiltersToSearch is the query string for a filter set and a page: ''
// when none is set, otherwise starting with '?'. Page 1 has no parameter.
export function objectFiltersToSearch(
	filters: ObjectFilters,
	page = 1,
): string {
	const params = new URLSearchParams();

	for (const name of filters.usedBy) params.append('used_by', name);
	if (filters.createdBy) params.set('created_by', filters.createdBy);
	if (filters.createdSince) params.set('created_since', filters.createdSince);
	if (filters.updatedSince) params.set('updated_since', filters.updatedSince);

	if (page > 1) params.set('page', String(page));

	const search = params.toString();

	return search === '' ? '' : `?${search}`;
}

export function hasActiveFilters(filters: ObjectFilters): boolean {
	return (
		filters.usedBy.length > 0 ||
		Boolean(filters.createdBy) ||
		Boolean(filters.createdSince) ||
		Boolean(filters.updatedSince)
	);
}

// startOfDay is the chosen day's first moment in the browser's own time
// zone, which is how the date is shown everywhere else.
function startOfDay(day: string): number {
	return new Date(`${day}T00:00:00`).getTime();
}

function since(timestamp: string | undefined, day: string | undefined) {
	if (!day) return true;
	if (!timestamp) return false;

	return new Date(timestamp).getTime() >= startOfDay(day);
}

// matchesFilters applies the filters the browser owns; usedBy was already
// applied by the load function through the API.
export function matchesFilters(
	object: ObjectMetadata,
	filters: ObjectFilters,
): boolean {
	if (filters.createdBy) {
		if (!object.created_by) return false;
		if (actorName(object.created_by) !== filters.createdBy) return false;
	}

	return (
		since(object.created_at, filters.createdSince) &&
		since(object.updated_at, filters.updatedSince)
	);
}

// unionObjects merges per-consumer lists into one, each object once, in the
// API's own slug order.
export function unionObjects(lists: ObjectMetadata[][]): ObjectMetadata[] {
	const byId = new Map<string, ObjectMetadata>();

	for (const list of lists) {
		for (const object of list) byId.set(object.id, object);
	}

	return [...byId.values()].sort((a, b) => a.slug.localeCompare(b.slug));
}
