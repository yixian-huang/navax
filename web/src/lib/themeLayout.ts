import type { PageSettings, ThemeLayout, LayoutTemplate, Density } from '@/api/types';

type CategoryStyle = PageSettings['layout']['categoryStyle'];

function pickEnum<T extends string>(knob: { default: string; allowed: string[]; locked: boolean }, current: T): T {
  if (knob.locked || !knob.allowed.includes(current)) {
    return knob.default as T;
  }
  return current;
}

export function applyThemeLayout(layout: PageSettings['layout'], spec?: ThemeLayout | null): PageSettings['layout'] {
  if (!spec) return layout;
  let columns = layout.columns;
  if (spec.columns.locked) {
    columns = spec.columns.default;
  } else if (columns < spec.columns.min) {
    columns = spec.columns.min;
  } else if (columns > spec.columns.max) {
    columns = spec.columns.max;
  }
  return {
    ...layout,
    template: pickEnum(spec.template, layout.template) as LayoutTemplate,
    density: pickEnum(spec.density, layout.density) as Density,
    categoryStyle: pickEnum(spec.categoryStyle, layout.categoryStyle) as CategoryStyle,
    columns,
  };
}
