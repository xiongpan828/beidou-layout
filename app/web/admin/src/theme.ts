/**
 * B 端主题：暖中性色 + 陶土橘强调色，偏「工具」——
 * 小圆角、紧凑行高、无阴影，强调色只出现在主操作与选中态。
 * 新项目换品牌色时改这里和 global.css 的 :root 变量即可。
 */
import { theme } from 'antd';

export const ADMIN_THEME = {
  algorithm: theme.defaultAlgorithm,
  token: {
    colorPrimary: '#C05A34',
    colorInfo: '#C05A34',
    colorSuccess: '#6F8A6A',
    colorWarning: '#C9922F',
    colorError: '#C0392B',
    colorLink: '#C05A34',
    colorText: '#1B1A17',
    colorTextSecondary: '#4A4842',
    colorTextTertiary: '#8B8579',
    colorBorder: '#E7E4DC',
    colorBorderSecondary: '#F1EEE7',
    colorBgBase: '#F7F7F5',
    colorBgContainer: '#FFFFFF',
    colorBgElevated: '#FFFFFF',
    borderRadius: 8,
    controlHeight: 34,
    fontSize: 13,
    fontFamily:
      "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif",
  },
  components: {
    Button: { primaryShadow: 'none', defaultShadow: 'none', fontWeight: 500 },
    Table: {
      headerBg: '#F6F5F1',
      headerColor: '#4A4842',
      headerSplitColor: 'transparent',
      rowHoverBg: '#F8F7F3',
      borderColor: '#EFECE5',
      cellPaddingBlock: 11,
      cellPaddingInline: 14,
      footerBg: 'transparent',
    },
    Card: { headerBg: 'transparent', paddingLG: 20 },
    Modal: { contentBg: '#FFFFFF', headerBg: '#FFFFFF' },
    Descriptions: { labelBg: '#FAF9F6' },
  },
};
