import './style.css';
import '@fontsource/nunito/400.css';
import '@fontsource/nunito/700.css';
import { registerSW } from 'virtual:pwa-register';
import { renderHome } from './ui/home';
import { renderPuzzle } from './ui/puzzle';
import { onRoute } from './ui/router';
import { applyTheme, watchSystemTheme } from './ui/settings';
import { loadSave } from './ui/storage';

registerSW({ immediate: true });
loadSave();
applyTheme();
watchSystemTheme();

const app = document.getElementById('app') as HTMLElement;
onRoute((route) => {
  if (route.name === 'puzzle') renderPuzzle(app, route.themeId, route.index);
  else renderHome(app);
});
