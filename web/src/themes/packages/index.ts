import { themeRegistry } from '@/themes/registry';
import { slateTheme } from '@/themes/packages/slate';
import { slateDarkTheme } from '@/themes/packages/slate-dark';
import { noirTheme } from '@/themes/packages/noir';
import { sakuraTheme } from '@/themes/packages/sakura';
import { orbitTheme } from '@/themes/packages/orbit';
import { terminalTheme } from '@/themes/packages/terminal';

themeRegistry.registerAll([
  slateTheme,
  slateDarkTheme,
  noirTheme,
  sakuraTheme,
  orbitTheme,
  terminalTheme,
]);
