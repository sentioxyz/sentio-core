const defaultTheme = require('tailwindcss/defaultTheme')

module.exports = {
  darkMode: 'selector',
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    colors: ({ colors }) => ({
      inherit: colors.inherit,
      current: colors.current,
      transparent: colors.transparent,
      black: colors.black,
      white: 'rgba(var(--white))',
      slate: colors.slate,
      gray: {
        50: 'rgba(var(--gray-50))',
        100: 'rgba(var(--gray-100))',
        200: 'rgba(var(--gray-200))',
        300: 'rgba(var(--gray-300))',
        400: 'rgba(var(--gray-400))',
        500: 'rgba(var(--gray-500))',
        600: 'rgba(var(--gray-600))',
        700: 'rgba(var(--gray-700))',
        800: 'rgba(var(--gray-800))',
        900: 'rgba(var(--gray-900))',
        DEFAULT: 'rgba(var(--gray-600))'
      },
      zinc: colors.zinc,
      neutral: colors.neutral,
      stone: colors.stone,
      red: {
        50: 'rgba(var(--red-50))',
        100: 'rgba(var(--red-100))',
        200: 'rgba(var(--red-200))',
        300: 'rgba(var(--red-300))',
        400: 'rgba(var(--red-400))',
        500: 'rgba(var(--red-500))',
        600: 'rgba(var(--red-600))',
        700: 'rgba(var(--red-700))',
        800: 'rgba(var(--red-800))',
        900: 'rgba(var(--red-900))',
        DEFAULT: 'rgba(var(--red-600))'
      },
      orange: {
        50: 'rgba(var(--orange-50))',
        100: 'rgba(var(--orange-100))',
        200: 'rgba(var(--orange-200))',
        300: 'rgba(var(--orange-300))',
        400: 'rgba(var(--orange-400))',
        500: 'rgba(var(--orange-500))',
        600: 'rgba(var(--orange-600))',
        700: 'rgba(var(--orange-700))',
        800: 'rgba(var(--orange-800))',
        900: 'rgba(var(--orange-900))',
        DEFAULT: 'rgba(var(--orange-600))'
      },
      amber: colors.amber,
      yellow: {
        50: 'rgba(var(--yellow-50))',
        100: 'rgba(var(--yellow-100))',
        200: 'rgba(var(--yellow-200))',
        300: 'rgba(var(--yellow-300))',
        400: 'rgba(var(--yellow-400))',
        500: 'rgba(var(--yellow-500))',
        600: 'rgba(var(--yellow-600))',
        700: 'rgba(var(--yellow-700))',
        800: 'rgba(var(--yellow-800))',
        900: 'rgba(var(--yellow-900))',
        DEFAULT: 'rgba(var(--yellow-600))'
      },
      lime: colors.lime,
      green: colors.green,
      emerald: colors.emerald,
      teal: colors.teal,
      cyan: {
        50: 'rgba(var(--cyan-50))',
        100: 'rgba(var(--cyan-100))',
        200: 'rgba(var(--cyan-200))',
        300: 'rgba(var(--cyan-300))',
        400: 'rgba(var(--cyan-400))',
        500: 'rgba(var(--cyan-500))',
        600: 'rgba(var(--cyan-600))',
        700: 'rgba(var(--cyan-700))',
        800: 'rgba(var(--cyan-800))',
        900: 'rgba(var(--cyan-900))',
        DEFAULT: 'rgba(var(--cyan-600))'
      },
      sky: colors.sky,
      blue: colors.blue,
      indigo: colors.indigo,
      violet: colors.violet,
      purple: {
        50: 'rgba(var(--purple-50))',
        100: 'rgba(var(--purple-100))',
        200: 'rgba(var(--purple-200))',
        300: 'rgba(var(--purple-300))',
        400: 'rgba(var(--purple-400))',
        500: 'rgba(var(--purple-500))',
        600: 'rgba(var(--purple-600))',
        700: 'rgba(var(--purple-700))',
        800: 'rgba(var(--purple-800))',
        900: 'rgba(var(--purple-900))',
        DEFAULT: 'rgba(var(--purple-600))'
      },
      fuchsia: colors.fuchsia,
      pink: colors.pink,
      rose: colors.rose
    }),
    extend: {
      colors: {
        primary: {
          50: 'rgba(var(--primary-50))',
          100: 'rgba(var(--primary-100))',
          200: 'rgba(var(--primary-200))',
          300: 'rgba(var(--primary-300))',
          400: 'rgba(var(--primary-400))',
          500: 'rgba(var(--primary-500))',
          600: 'rgba(var(--primary-600))',
          700: 'rgba(var(--primary-700))',
          800: 'rgba(var(--primary-800))',
          900: 'rgba(var(--primary-900))',
          DEFAULT: 'rgba(var(--primary-600))'
        },
        nav: 'rgba(var(--gray-100))',
        sidebar: 'rgba(var(--sidebar-background))',
        'daybreak-blue': {
          50: 'rgba(var(--daybreak-blue-50))',
          100: 'rgba(var(--daybreak-blue-100))',
          200: 'rgba(var(--daybreak-blue-200))',
          300: 'rgba(var(--daybreak-blue-300))',
          400: 'rgba(var(--daybreak-blue-400))',
          500: 'rgba(var(--daybreak-blue-500))',
          600: 'rgba(var(--daybreak-blue-600))',
          700: 'rgba(var(--daybreak-blue-700))',
          800: 'rgba(var(--daybreak-blue-800))',
          900: 'rgba(var(--daybreak-blue-900))',
          DEFAULT: 'rgba(var(--daybreak-blue-600))'
        },
        'lake-blue': {
          50: 'rgba(var(--lake-blue-50))',
          100: 'rgba(var(--lake-blue-100))',
          200: 'rgba(var(--lake-blue-200))',
          300: 'rgba(var(--lake-blue-300))',
          400: 'rgba(var(--lake-blue-400))',
          500: 'rgba(var(--lake-blue-500))',
          600: 'rgba(var(--lake-blue-600))',
          700: 'rgba(var(--lake-blue-700))',
          800: 'rgba(var(--lake-blue-800))',
          900: 'rgba(var(--lake-blue-900))',
          DEFAULT: 'rgba(var(--lake-blue-600))'
        },
        'sentio-gray': {
          50: 'rgba(var(--sentio-gray-50))',
          100: 'rgba(var(--sentio-gray-100))',
          200: 'rgba(var(--sentio-gray-200))',
          300: 'rgba(var(--sentio-gray-300))',
          400: 'rgba(var(--sentio-gray-400))',
          500: 'rgba(var(--sentio-gray-500))',
          600: 'rgba(var(--sentio-gray-600))',
          700: 'rgba(var(--sentio-gray-700))',
          800: 'rgba(var(--sentio-gray-800))',
          900: 'rgba(var(--sentio-gray-900))',
          DEFAULT: 'rgba(var(--sentio-gray-600))'
        },
        'deep-purple': {
          50: 'rgba(var(--deep-purple-50))',
          100: 'rgba(var(--deep-purple-100))',
          200: 'rgba(var(--deep-purple-200))',
          300: 'rgba(var(--deep-purple-300))',
          400: 'rgba(var(--deep-purple-400))',
          500: 'rgba(var(--deep-purple-500))',
          600: 'rgba(var(--deep-purple-600))',
          700: 'rgba(var(--deep-purple-700))',
          800: 'rgba(var(--deep-purple-800))',
          900: 'rgba(var(--deep-purple-900))',
          DEFAULT: 'rgba(var(--deep-purple-600))'
        },
        magenta: {
          50: 'rgba(var(--magenta-50))',
          100: 'rgba(var(--magenta-100))',
          200: 'rgba(var(--magenta-200))',
          300: 'rgba(var(--magenta-300))',
          400: 'rgba(var(--magenta-400))',
          500: 'rgba(var(--magenta-500))',
          600: 'rgba(var(--magenta-600))',
          700: 'rgba(var(--magenta-700))',
          800: 'rgba(var(--magenta-800))',
          900: 'rgba(var(--magenta-900))',
          DEFAULT: 'rgba(var(--magenta-600))'
        },
        'text-foreground': 'rgba(var(--text-foreground))',
        'text-foreground-secondary': 'rgba(var(--text-foreground-secondary))',
        'text-foreground-disabled': 'rgba(var(--text-foreground-disabled))',
        'text-background': 'rgba(var(--text-background))',
        'border-color': 'rgba(var(--border-color))',
        'border-color-secondary': 'rgba(var(--border-color), 0.5)',
        'input-border-color': 'rgba(var(--input-border-color))',
        'input-background': 'rgba(var(--input-background))',
        'navbar-foreground': 'rgba(var(--navbar-foreground))',
        'navbar-hover-foreground': 'rgba(var(--navbar-hover-foreground))',
        'navbar-hover-background': 'rgba(var(--navbar-hover-background))',
        'navbar-selected-background': 'rgba(var(--navbar-selected-background))'
      },
      borderColor: {
        DEFAULT: 'rgba(var(--border-color))'
      },
      divideColor: {
        DEFAULT: 'rgba(var(--border-color))'
      },
      zIndex: {
        nav: '2',
        tooltip: '90'
      },
      fontSize: {
        ichart: ['0.625rem', '1rem'], // chart font size 10px 16px
        icontent: ['0.8125rem', '1.125rem'], // content font size 12px 16px
        ilabel: ['0.8125rem', '1.125rem'], // content header font size 13px 18px
        ititle: ['1.125rem', '1.75rem'] // first class header font size 18px 28px
      },
      fontWeight: {
        icontent: '400',
        ilabel: '500'
      },
      height: {
        4.5: '1.125rem'
      },
      width: {
        4.5: '1.125rem'
      },
      fontFamily: {
        mono: ['Menlo', ...defaultTheme.fontFamily.mono],
        code: ['Fira Code', 'Fira Mono', 'Menlo', 'Consolas', 'DejaVu Sans Mono', ...defaultTheme.fontFamily.mono]
      },
      keyframes: {
        'fade-in': {
          '0%': {
            opacity: '0',
            transform: 'translateY(20px)'
          },
          '100%': {
            opacity: '1',
            transform: 'translateY(0)'
          }
        },
        float: {
          '0%, 100%': {
            transform: 'translateY(10px)'
          },
          '50%': {
            transform: 'translateY(-10px)'
          }
        },
        'bounce-x': {
          '0%, 100%': {
            transform: 'translateX(0)'
          },
          '50%': {
            transform: 'translateX(4px)'
          }
        }
      },
      animation: {
        'fade-in': 'fade-in 0.6s ease-out forwards',
        float: 'float 3s ease-in-out infinite',
        'bounce-x': 'bounce-x 0.6s ease-in-out infinite'
      }
    },
    patterns: {
      opacities: {
        100: '1',
        80: '.80',
        60: '.60',
        40: '.40',
        20: '.20',
        10: '.10',
        5: '.05'
      },
      sizes: {
        0.5: '0.125rem',
        1: '0.25rem',
        2: '0.5rem',
        4: '1rem',
        6: '1.5rem',
        8: '2rem',
        16: '4rem',
        20: '5rem',
        24: '6rem',
        32: '8rem'
      }
    }
  },
  plugins: []
}
