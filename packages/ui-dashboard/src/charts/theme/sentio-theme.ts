// Inlined here (was `lib/fonts` in the app, which prepends a next/font face).
// The app applies its custom font globally via CSS; the ECharts theme just needs
// a sane sans stack. Exported so EchartsBase can reuse it for axis-name labels.
export const sansFontFamily =
  'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif'
export { sentioColors } from './sentio-colors'
import { sentioColors } from './sentio-colors'

// Matches --text-foreground-secondary in app/styles/theme-variables.css
const textSecondaryLight = '#625d75'
const textSecondaryDark = '#b7b4c7'

export const sentioTheme = {
  color: sentioColors.light.classic,
  backgroundColor: 'rgba(0,0,0,0)',
  textStyle: {
    fontSize: 11,
    fontFamily: sansFontFamily,
    color: textSecondaryLight
  },
  title: {
    textStyle: {
      color: textSecondaryLight
    },
    subtextStyle: {
      color: textSecondaryLight
    }
  },
  line: {
    itemStyle: {
      borderWidth: 1
    },
    lineStyle: {
      width: 2
    },
    symbolSize: 4,
    symbol: 'emptyCircle',
    smooth: false
  },
  radar: {
    itemStyle: {
      borderWidth: 1
    },
    lineStyle: {
      width: 2
    },
    symbolSize: 4,
    symbol: 'emptyCircle',
    smooth: false
  },
  bar: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  pie: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    },
    label: {
      textBorderWidth: 0,
      textBorderColor: 'transparent'
    }
  },
  scatter: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  boxplot: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  parallel: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  sankey: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  funnel: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  gauge: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  candlestick: {
    itemStyle: {
      color: '#eb5454',
      color0: '#47b262',
      borderColor: '#eb5454',
      borderColor0: '#47b262',
      borderWidth: 1
    }
  },
  graph: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    },
    lineStyle: {
      width: 1,
      color: '#aaaaaa'
    },
    symbolSize: 4,
    symbol: 'emptyCircle',
    smooth: false,
    color: [
      '#2e71db',
      '#8dc869',
      '#ffdc2d',
      '#f05a4d',
      '#56bce5',
      '#73ba46',
      '#fe9f05',
      '#a452d7',
      '#a65a8b'
    ],
    label: {
      color: '#ebeff3'
    }
  },
  map: {
    itemStyle: {
      areaColor: '#eee',
      borderColor: '#444',
      borderWidth: 0.5
    },
    label: {
      color: '#000'
    },
    emphasis: {
      itemStyle: {
        areaColor: 'rgba(255,215,0,0.8)',
        borderColor: '#444',
        borderWidth: 1
      },
      label: {
        color: 'rgb(100,0,0)'
      }
    }
  },
  geo: {
    itemStyle: {
      areaColor: '#eee',
      borderColor: '#444',
      borderWidth: 0.5
    },
    label: {
      color: '#000'
    },
    emphasis: {
      itemStyle: {
        areaColor: 'rgba(255,215,0,0.8)',
        borderColor: '#444',
        borderWidth: 1
      },
      label: {
        color: 'rgb(100,0,0)'
      }
    }
  },
  categoryAxis: {
    axisLine: {
      show: true,
      lineStyle: {
        // matches CSS --border-color in light mode (rgb(235,239,243))
        color: '#EBEFF3'
      }
    },
    axisTick: {
      show: true,
      lineStyle: {
        color: '#EBEFF3'
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryLight,
      fontWeight: 'normal'
    },
    splitLine: {
      show: false,
      lineStyle: {
        color: ['#E0E6F1']
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  valueAxis: {
    axisLine: {
      show: false,
      lineStyle: {
        color: textSecondaryLight
      }
    },
    axisTick: {
      show: false,
      lineStyle: {
        color: textSecondaryLight
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryLight,
      fontWeight: 'normal'
    },
    splitLine: {
      show: true,
      lineStyle: {
        color: 'rgba(228, 232, 237, 0.3)'
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  logAxis: {
    axisLine: {
      show: false,
      lineStyle: {
        color: textSecondaryLight
      }
    },
    axisTick: {
      show: false,
      lineStyle: {
        color: textSecondaryLight
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryLight
    },
    splitLine: {
      show: true,
      lineStyle: {
        color: 'rgba(89, 93, 97, 0.8)'
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  timeAxis: {
    axisLine: {
      show: true,
      lineStyle: {
        // matches CSS --border-color in light mode (rgb(235,239,243))
        color: '#EBEFF3'
      }
    },
    axisTick: {
      show: true,
      lineStyle: {
        color: '#EBEFF3'
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryLight
    },
    splitLine: {
      show: false,
      lineStyle: {
        color: ['#E0E6F1']
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  toolbox: {
    iconStyle: {
      borderColor: '#999999'
    },
    emphasis: {
      iconStyle: {
        borderColor: '#666666'
      }
    }
  },
  legend: {
    textStyle: {
      color: textSecondaryLight,
      fontSize: 10
    },
    pageIconColor: '#4E5969',
    pageIconInactiveColor: '#C9CDD4',
    pageTextStyle: {
      color: textSecondaryLight
    }
  },
  tooltip: {
    axisPointer: {
      lineStyle: {
        color: '#e0e0e0',
        width: 1
      },
      crossStyle: {
        color: '#e0e0e0',
        width: 1
      }
    }
  },
  timeline: {
    lineStyle: {
      color: '#dae1f5',
      width: 2
    },
    itemStyle: {
      color: '#a4b1d7',
      borderWidth: 1
    },
    controlStyle: {
      color: '#a4b1d7',
      borderColor: '#a4b1d7',
      borderWidth: 1
    },
    checkpointStyle: {
      color: '#316bf3',
      borderColor: '#ffffff'
    },
    label: {
      color: '#a4b1d7'
    },
    emphasis: {
      itemStyle: {
        color: '#ffffff'
      },
      controlStyle: {
        color: '#a4b1d7',
        borderColor: '#a4b1d7',
        borderWidth: 1
      },
      label: {
        color: '#a4b1d7'
      }
    }
  },
  visualMap: {
    color: ['#bf444c', '#d88273', '#f6efa6']
  },
  dataZoom: {
    handleSize: 'undefined%',
    textStyle: {}
  },
  markPoint: {
    label: {
      color: '#ebeff3'
    },
    emphasis: {
      label: {
        color: '#ebeff3'
      }
    }
  }
}

export const sentioThemeDark = {
  color: sentioColors.dark.classic,
  backgroundColor: 'rgba(0,0,0,0)',
  textStyle: {
    fontSize: 11,
    fontFamily: sansFontFamily,
    textBorderWidth: 0,
    textBorderColor: 'transparent',
    color: textSecondaryDark
  },
  title: {
    textStyle: {
      color: textSecondaryDark
    },
    subtextStyle: {
      color: textSecondaryDark
    }
  },
  line: {
    itemStyle: {
      borderWidth: 1
    },
    lineStyle: {
      width: 2
    },
    symbolSize: 4,
    symbol: 'emptyCircle',
    smooth: false
  },
  radar: {
    itemStyle: {
      borderWidth: 1
    },
    lineStyle: {
      width: 2
    },
    symbolSize: 4,
    symbol: 'emptyCircle',
    smooth: false
  },
  bar: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  pie: {
    itemStyle: {
      borderWidth: 0,
      borderColor: 'transparent'
    },
    label: {
      textBorderWidth: 0,
      textBorderColor: 'transparent',
      color: textSecondaryDark
    }
  },
  scatter: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  boxplot: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  parallel: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  sankey: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  funnel: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  gauge: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    }
  },
  candlestick: {
    itemStyle: {
      color: '#eb5454',
      color0: '#47b262',
      borderColor: '#eb5454',
      borderColor0: '#47b262',
      borderWidth: 1
    }
  },
  graph: {
    itemStyle: {
      borderWidth: 0,
      borderColor: '#ccc'
    },
    lineStyle: {
      width: 1,
      color: '#aaaaaa'
    },
    symbolSize: 4,
    symbol: 'emptyCircle',
    smooth: false,
    color: [
      '#2e71db',
      '#a8d58d',
      '#ffe355',
      '#f05a4d',
      '#56bce5',
      '#73ba46',
      '#ff9f05',
      '#ad56e2',
      '#e97ec2'
    ],
    label: {
      color: '#ebeff3'
    }
  },
  map: {
    itemStyle: {
      areaColor: '#eee',
      borderColor: '#444',
      borderWidth: 0.5
    },
    label: {
      color: '#000'
    },
    emphasis: {
      itemStyle: {
        areaColor: 'rgba(255,215,0,0.8)',
        borderColor: '#444',
        borderWidth: 1
      },
      label: {
        color: 'rgb(100,0,0)'
      }
    }
  },
  geo: {
    itemStyle: {
      areaColor: '#eee',
      borderColor: '#444',
      borderWidth: 0.5
    },
    label: {
      color: '#000'
    },
    emphasis: {
      itemStyle: {
        areaColor: 'rgba(255,215,0,0.8)',
        borderColor: '#444',
        borderWidth: 1
      },
      label: {
        color: 'rgb(100,0,0)'
      }
    }
  },
  categoryAxis: {
    axisLine: {
      show: true,
      lineStyle: {
        // matches CSS --border-color in dark mode (gray-100 = rgb(66,66,72))
        color: '#424248'
      }
    },
    axisTick: {
      show: true,
      lineStyle: {
        color: '#424248'
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryDark,
      fontWeight: 'normal'
    },
    splitLine: {
      show: false,
      lineStyle: {
        color: ['#E0E6F1']
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  valueAxis: {
    axisLine: {
      show: false,
      lineStyle: {
        color: textSecondaryDark
      }
    },
    axisTick: {
      show: false,
      lineStyle: {
        color: textSecondaryDark
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryDark
    },
    splitLine: {
      show: true,
      lineStyle: {
        // softer gridline on the new dark canvas — barely visible
        color: 'rgba(255, 255, 255, 0.05)',
        width: 1,
        opacity: 0.4
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  logAxis: {
    axisLine: {
      show: false,
      lineStyle: {
        color: textSecondaryDark
      }
    },
    axisTick: {
      show: false,
      lineStyle: {
        color: textSecondaryDark
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryDark,
      fontWeight: 'normal'
    },
    splitLine: {
      show: true,
      lineStyle: {
        // softer gridline on the new dark canvas
        color: ['rgba(255, 255, 255, 0.05)'],
        width: 1,
        opacity: 0.4
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  timeAxis: {
    axisLine: {
      show: true,
      lineStyle: {
        // matches CSS --border-color in dark mode (gray-100 = rgb(66,66,72))
        color: '#424248'
      }
    },
    axisTick: {
      show: true,
      lineStyle: {
        color: '#424248'
      }
    },
    axisLabel: {
      show: true,
      color: textSecondaryDark
    },
    splitLine: {
      show: false,
      lineStyle: {
        color: ['#5d6165']
      }
    },
    splitArea: {
      show: false,
      areaStyle: {
        color: ['rgba(250,250,250,0.2)', 'rgba(210,219,238,0.2)']
      }
    }
  },
  toolbox: {
    iconStyle: {
      borderColor: '#999999'
    },
    emphasis: {
      iconStyle: {
        borderColor: '#666666'
      }
    }
  },
  legend: {
    textStyle: {
      color: textSecondaryDark
    },
    pageIconColor: '#909399',
    pageIconInactiveColor: '#606266',
    pageTextStyle: {
      color: textSecondaryDark
    }
  },
  tooltip: {
    axisPointer: {
      lineStyle: {
        color: '#e0e0e0',
        width: 1
      },
      crossStyle: {
        color: '#e0e0e0',
        width: 1
      }
    },
    backgroundColor: '#202020',
    textStyle: {
      color: textSecondaryDark
    }
  },
  timeline: {
    lineStyle: {
      color: '#dae1f5',
      width: 2
    },
    itemStyle: {
      color: '#a4b1d7',
      borderWidth: 1
    },
    controlStyle: {
      color: '#a4b1d7',
      borderColor: '#a4b1d7',
      borderWidth: 1
    },
    checkpointStyle: {
      color: '#316bf3',
      borderColor: '#ffffff'
    },
    label: {
      color: '#a4b1d7'
    },
    emphasis: {
      itemStyle: {
        color: '#ffffff'
      },
      controlStyle: {
        color: '#a4b1d7',
        borderColor: '#a4b1d7',
        borderWidth: 1
      },
      label: {
        color: '#a4b1d7'
      }
    }
  },
  visualMap: {
    color: ['#bf444c', '#d88273', '#f6efa6']
  },
  dataZoom: {
    handleSize: 'undefined%',
    textStyle: {}
  },
  markPoint: {
    label: {
      color: '#ebeff3'
    },
    emphasis: {
      label: {
        color: '#ebeff3'
      }
    }
  }
}
