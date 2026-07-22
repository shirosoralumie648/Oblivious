export const releaseProjectionDigest = 'sha256:825ba0c66537132811fa280a62f761fb2b95eda35d69975d6df715f8e8bb1c38' as const;

export const releaseCapabilityProjection = [
  {
    capabilityId: 'fixture.conditional',
    disposition: 'conditional',
    navigationDisposition: 'conditional',
    reasonCode: 'dependency_unproven'
  },
  {
    capabilityId: 'fixture.excluded',
    disposition: 'excluded',
    navigationDisposition: 'hidden',
    reasonCode: 'capability_excluded'
  },
  {
    capabilityId: 'fixture.users',
    disposition: 'committed',
    navigationDisposition: 'visible'
  }
] as const;

export const releaseSurfaceProjection = [
  {
    canonicalSource: 'scripts/testdata/frontend-surface/production',
    capabilityIds: ['fixture.conditional', 'fixture.excluded', 'fixture.users'],
    consumer: 'frontend-transport-inventory',
    disposition: 'committed',
    navigationDisposition: 'visible',
    surfaceId: 'frontend'
  }
] as const;
