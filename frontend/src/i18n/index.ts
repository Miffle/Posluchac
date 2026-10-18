import { computed } from 'vue'
import { useSettingsStore } from '../store/settings'

export const translations = {
  ru: {
    nav: {
      search: 'Поиск',
      library: 'Библиотека',
      downloads: 'Загрузки',
      playlists: 'Плейлисты',
      settings: 'Настройки',
      newVersion: 'Новая версия!',
      checkUpdates: 'Проверить наличие обновлений',
    },
    settings: {
      title: 'Настройки',
      subtitle: 'Параметры плеера и интерфейса Posluchac',
      languageTitle: 'Язык интерфейса',
      languageDesc: 'Выберите предпочитаемый язык для отображения приложения',
      ru: 'Русский',
      en: 'English',
      ruNative: 'Русский',
      enNative: 'English',
      storageTitle: 'Хранилище и загрузки',
      storageDesc: 'Каталог, в который загружаются музыкальные файлы и альбомы',
      openFolder: 'Открыть папку с музыкой',
      aboutTitle: 'О приложении',
      aboutDesc: 'Современный аудиоплеер с поиском, предварительным прослушиванием и скачиванием музыки через торренты.',
      version: 'Версия',
      developer: 'Разработчик',
      checkUpdateBtn: 'Проверить обновления',
      checkingUpdate: 'Проверка...',
      githubRepo: 'GitHub Репозиторий',
      themeTitle: 'Внешний вид',
      themeDesc: 'Тёмная тема Posluchac оптимизирована для комфортного прослушивания',
    },
    player: {
      noTrack: 'Нет трека',
      expandCover: 'Развернуть обложку на весь экран',
      collapseCover: 'Свернуть (Esc)',
      repeatOff: 'Повтор выключен (нажмите для повтора плейлиста)',
      repeatAll: 'Повтор плейлиста (нажмите для повтора одной песни)',
      repeatOne: 'Повтор одной песни (нажмите для выключения)',
      shuffleOn: 'Случайный порядок (вкл)',
      shuffleOff: 'Случайный порядок (выкл)',
      mute: 'Выключить звук',
      unmute: 'Включить звук',
      nowPlaying: 'Сейчас играет',
      pressEsc: 'Нажмите Esc или ✕ чтобы свернуть',
      noCover: 'Без обложки',
    },
    library: {
      title: 'Библиотека',
      allTracks: 'Все треки',
      artists: 'Исполнители',
      favourites: 'Избранное',
      searchPlaceholder: 'Поиск по названию, артисту, альбому...',
      rescan: 'Пересканировать папку',
      empty: 'Библиотека пуста',
      emptyDesc: 'Скачайте музыку через Поиск или скопируйте аудиофайлы в папку Музыка',
    },
    common: {
      play: 'Воспроизвести',
      pause: 'Пауза',
      close: 'Закрыть',
      delete: 'Удалить',
      cancel: 'Отмена',
      save: 'Сохранить',
    },
  },
  en: {
    nav: {
      search: 'Search',
      library: 'Library',
      downloads: 'Downloads',
      playlists: 'Playlists',
      settings: 'Settings',
      newVersion: 'New Version!',
      checkUpdates: 'Check for updates',
    },
    settings: {
      title: 'Settings',
      subtitle: 'Posluchac preferences and interface options',
      languageTitle: 'Interface Language',
      languageDesc: 'Select your preferred language for the application display',
      ru: 'Russian',
      en: 'English',
      ruNative: 'Русский',
      enNative: 'English',
      storageTitle: 'Storage & Downloads',
      storageDesc: 'Directory where downloaded music tracks and albums are saved',
      openFolder: 'Open Music Folder',
      aboutTitle: 'About Application',
      aboutDesc: 'Modern audio player with torrent search, instant streaming preview, and library management.',
      version: 'Version',
      developer: 'Developer',
      checkUpdateBtn: 'Check for Updates',
      checkingUpdate: 'Checking...',
      githubRepo: 'GitHub Repository',
      themeTitle: 'Appearance',
      themeDesc: 'Posluchac dark theme is tuned for comfortable listening',
    },
    player: {
      noTrack: 'No track',
      expandCover: 'Expand cover to full screen',
      collapseCover: 'Collapse (Esc)',
      repeatOff: 'Repeat off (click to repeat playlist)',
      repeatAll: 'Repeat playlist (click to repeat one song)',
      repeatOne: 'Repeat one song (click to turn off)',
      shuffleOn: 'Shuffle enabled',
      shuffleOff: 'Shuffle disabled',
      mute: 'Mute',
      unmute: 'Unmute',
      nowPlaying: 'Now Playing',
      pressEsc: 'Press Esc or ✕ to exit',
      noCover: 'No Cover Art',
    },
    library: {
      title: 'Library',
      allTracks: 'All Tracks',
      artists: 'Artists',
      favourites: 'Favorites',
      searchPlaceholder: 'Search by title, artist, album...',
      rescan: 'Rescan Folder',
      empty: 'Library is empty',
      emptyDesc: 'Download music via Search or place audio files into your Music folder',
    },
    common: {
      play: 'Play',
      pause: 'Pause',
      close: 'Close',
      delete: 'Delete',
      cancel: 'Cancel',
      save: 'Save',
    },
  },
}

export type TranslationKey = keyof typeof translations.ru

export function useI18n() {
  const settings = useSettingsStore()

  function t(path: string, fallback = ''): string {
    const lang = settings.language || 'ru'
    const dict = translations[lang] || translations.ru

    const parts = path.split('.')
    let current: any = dict
    for (const part of parts) {
      if (current && typeof current === 'object' && part in current) {
        current = current[part]
      } else {
        // Fallback to Russian if missing in English
        let fallbackCur: any = translations.ru
        for (const fbPart of parts) {
          if (fallbackCur && typeof fallbackCur === 'object' && fbPart in fallbackCur) {
            fallbackCur = fallbackCur[fbPart]
          } else {
            return fallback || path
          }
        }
        return typeof fallbackCur === 'string' ? fallbackCur : (fallback || path)
      }
    }

    return typeof current === 'string' ? current : (fallback || path)
  }

  const currentLang = computed(() => settings.language)

  return {
    t,
    currentLang,
    setLanguage: settings.setLanguage,
  }
}
