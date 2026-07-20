// Set to false to disable demo mode even in DEV builds
const DEMO_TOGGLE = false;

export const DEMO_ENABLED: boolean = import.meta.env.DEV && DEMO_TOGGLE;
