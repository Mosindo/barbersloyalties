import { createTamagui } from 'tamagui'

const config = createTamagui({
  themes: {
    light: {
      background: '#f7f3ea',
      color: '#1f1b16',
      primary: '#0f766e',
      secondary: '#b45309'
    }
  },
  tokens: {
    color: {
      background: '#f7f3ea',
      text: '#1f1b16',
      primary: '#0f766e',
      secondary: '#b45309'
    },
    size: {
      0: 0,
      1: 4,
      2: 8,
      3: 12,
      4: 16,
      5: 20,
      6: 24,
      7: 32
    },
    radius: {
      0: 0,
      1: 6,
      2: 10,
      3: 14
    },
    space: {
      0: 0,
      1: 4,
      2: 8,
      3: 12,
      4: 16,
      5: 20,
      6: 24,
      7: 32
    },
    zIndex: {
      0: 0,
      1: 10,
      2: 20,
      3: 30
    }
  },
  fonts: {
    body: {
      family: 'System',
      size: {
        1: 14,
        2: 16,
        3: 18
      },
      lineHeight: {
        1: 20,
        2: 22,
        3: 26
      },
      weight: {
        4: '400',
        6: '600',
        7: '700'
      },
      letterSpacing: {
        1: 0,
        2: 0,
        3: 0
      }
    }
  }
})

export type AppTamaguiConfig = typeof config

export default config
