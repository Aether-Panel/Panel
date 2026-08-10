import { sileo as sileoRaw } from "sileo";

const errorOptions = (opts: Parameters<typeof sileoRaw.error>[0]) => ({
	position: "top-center" as const,
	...opts,
});

export const sileo = {
	show: sileoRaw.show,
	success: sileoRaw.success,
	error: (opts: Parameters<typeof sileoRaw.error>[0]) => sileoRaw.error(errorOptions(opts)),
	warning: (opts: Parameters<typeof sileoRaw.warning>[0]) => sileoRaw.warning(errorOptions(opts)),
	info: sileoRaw.info,
	action: sileoRaw.action,
	promise: sileoRaw.promise,
	dismiss: sileoRaw.dismiss,
	clear: sileoRaw.clear,
};