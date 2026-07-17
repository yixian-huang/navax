import { themeRegistry } from '@/themes/registry';
import { slateTheme } from '@/themes/packages/slate';
import { slateDarkTheme } from '@/themes/packages/slate-dark';
import { kyotoTheme } from '@/themes/packages/kyoto';
import { noirTheme } from '@/themes/packages/noir';
import { terracottaTheme } from '@/themes/packages/terracotta';
import { sakuraTheme } from '@/themes/packages/sakura';
import { mochiTheme } from '@/themes/packages/mochi';
import { pastelskyTheme } from '@/themes/packages/pastelsky';
import { monoTheme } from '@/themes/packages/mono';
import { cyberTheme } from '@/themes/packages/cyber';
import { orbitTheme } from '@/themes/packages/orbit';
import { terminalTheme } from '@/themes/packages/terminal';

themeRegistry.registerAll([
  slateTheme,
  slateDarkTheme,
  kyotoTheme,
  noirTheme,
  terracottaTheme,
  sakuraTheme,
  mochiTheme,
  pastelskyTheme,
  monoTheme,
  cyberTheme,
  orbitTheme,
  terminalTheme,
]);