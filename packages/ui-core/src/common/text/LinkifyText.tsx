import { linkifyUrlsToHtml } from 'linkify-urls'
import DOMPurify from 'dompurify'
import { memo } from 'react'
import { isString, isUndefined, isNull } from 'lodash'

if (DOMPurify?.addHook) {
  DOMPurify.addHook('afterSanitizeAttributes', function (node) {
    // set all elements owning target to target=_blank
    if ('target' in node) {
      node.setAttribute('target', '_blank')
    }
    // set non-HTML/MathML links to xlink:show=new
    if (
      !node.hasAttribute('target') &&
      (node.hasAttribute('xlink:href') || node.hasAttribute('href'))
    ) {
      node.setAttribute('xlink:show', 'new')
    }
  })
}

const renderTextWithColoredNumbers = (text: string) => {
  // Use word boundary and negative lookahead to avoid matching numbers in hexadecimal addresses
  // Support scientific notation (e.g., 1e-7, 2.5e+10)
  const numberRegex = /\b(\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\b/g

  return text.replace(numberRegex, (match, number, offset) => {
    // Check characters before and after the number to ensure it's not part of a hex address
    const before = text.charAt(offset - 1)
    const after = text.charAt(offset + match.length)

    // If preceded by 'x' or surrounded by hex characters, don't highlight
    if (before === 'x' || /[a-fA-F]/.test(before) || /[a-fA-F]/.test(after)) {
      return match
    }

    return `<span class="font-mono text-primary-500 dark:text-primary-700">${match}</span>`
  })
}

interface LinkifyTextProps {
  text: any
  className?: string
  isHighlightNumbers?: boolean
}

export const LinkifyText = memo(function LinkifyText({
  text,
  className,
  isHighlightNumbers
}: LinkifyTextProps) {
  if (isUndefined(text) || isNull(text)) {
    return null
  }
  if (!isString(text)) {
    if (text.toString) {
      return <span className={className}>{text.toString()}</span>
    }
    return null
  }
  const linkStr = linkifyUrlsToHtml(
    isHighlightNumbers ? renderTextWithColoredNumbers(text) : text,
    {
      attributes: {
        class: 'text-primary hover:underline',
        target: '_blank',
        rel: 'noopener noreferrer'
      }
    }
  )
  return (
    <span
      className={className}
      dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(linkStr) }}
    />
  )
})
