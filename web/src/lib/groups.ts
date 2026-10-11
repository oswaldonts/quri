// Display names of the category groups, in the order they are listed.
export const groupNames: Record<string, string> = {
  needs: 'Necesidades',
  lifestyle: 'Estilo de vida',
  growth: 'Crecimiento',
  obligations: 'Obligaciones',
  support: 'Apoyo',
  other: 'Otros',
  system: 'Cambios de moneda',
}

// The groups a user can assign (the system one is reserved).
export const userGroups = ['needs', 'lifestyle', 'growth', 'obligations', 'support', 'other'] as const

export type UserGroup = (typeof userGroups)[number]
