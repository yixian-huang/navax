import { PointerSensor } from '@dnd-kit/core';

export function shouldHandlePointerDown(event: PointerEvent): boolean {
  const target = event.target;
  if (!(target instanceof Element)) return true;
  return !target.closest('button, a, input, textarea, [data-no-dnd]');
}

export class EditorPointerSensor extends PointerSensor {
  static activators = [
    {
      eventName: 'onPointerDown' as const,
      handler: ({ nativeEvent }: { nativeEvent: PointerEvent }) => {
        if (!nativeEvent.isPrimary || nativeEvent.button !== 0) return false;
        return shouldHandlePointerDown(nativeEvent);
      },
    },
  ];
}
