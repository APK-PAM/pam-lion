export function truncateCenter(s, l) {
  if (s.length <= l) {
    return s
  }
  const centerIndex = Math.ceil(l / 2)
  return s.slice(0, centerIndex - 2) + '...' + s.slice(centerIndex + 1, l)
}

function createWatermarkDiv(content, {
  width = 300,
  height = 300,
  textAlign = 'center',
  textBaseline = 'middle',
  alpha = 0.5,
  font = '20px monaco, microsoft yahei',
  fillStyle = 'rgba(184, 184, 184, 0.8)',
  rotate = -45,
  zIndex = 1000
}) {
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')

  canvas.width = width
  canvas.height = height
  ctx.globalAlpha = 0.5

  ctx.font = font
  ctx.fillStyle = fillStyle
  ctx.textAlign = textAlign
  ctx.textBaseline = textBaseline
  ctx.globalAlpha = alpha

  ctx.translate(0.5 * width, 0.5 * height)
  ctx.rotate((rotate * Math.PI) / 180)

  function generateMultiLineText(_ctx, _text, _width, _lineHeight) {
    const words = _text.split('\n')
    let line = ''
    const x = 0
    let y = 0

    for (let n = 0; n < words.length; n++) {
      line = words[n]
      line = truncateCenter(line, 25)
      _ctx.fillText(line, x, y)
      y += _lineHeight
    }
  }

  generateMultiLineText(ctx, content, width, 24)

  const base64Url = canvas.toDataURL()
  const watermarkDiv = document.createElement('div')

  const styles = {
    position: 'absolute',
    display: 'block',
    visibility: 'visible',
    top: 0,
    left: 0,
    width: '100%',
    height: '100%',
    opacity: 1,
    'z-index': zIndex,
    'pointer-events': 'none',
    'background-repeat': 'repeat',
    'background-image': `url('${base64Url}')`
  }
  const layoutStyles = ['margin', 'padding', 'border']
  const directionStyles = ['top', 'right', 'bottom', 'left']
  layoutStyles.forEach(lay => {
    directionStyles.forEach(direction => {
      const key = `${lay}-${direction}`
      styles[key] = '0'
    })
  })
  const style = Object.keys(styles)
    .reduce((prev, key) => {
      return `${prev}${key}:${styles[key]};`
    }, '')
  watermarkDiv.setAttribute('style', style)
  watermarkDiv.classList.add('watermark')
  return { watermark: watermarkDiv, base64: base64Url }
}

export function canvasWaterMark({ container = document.body, content = 'JumpServer', settings = {}
} = {}) {
  container.style.position = 'relative'
  const res = createWatermarkDiv(content, settings)
  const watermarkDiv = res.watermark
  container.insertBefore(watermarkDiv, container.firstChild)

  // 监听 dom 节点的 style 属性变化
  const observer = new MutationObserver(mutations => {
    setTimeout(() => {
      container.removeChild(container.firstChild)
      // 这里不用再新建了，因为下面监听了 container 的子节点变化，会重新创建的
      // canvasWaterMark({container, content, settings});
    }, 100)
  })
  observer.observe(watermarkDiv, { childList: false, attributes: true, subtree: false })

  const containerObserver = new MutationObserver(mutations => {
    const removed = mutations.filter(m => m.type === 'childList' && m.removedNodes.length > 0)
    if (removed.length === 0) {
      return
    }
    const removedNodes = removed[0].removedNodes
    if (removedNodes.length === 0) {
      return
    }
    const removedHtml = removedNodes[0]['outerHTML']
    if (removedHtml.indexOf(res.base64) < 0) {
      return
    }
    setTimeout(() => {
      canvasWaterMark({ container, content, settings })
    }, 100)
  })
  containerObserver.observe(container, { childList: true, attributes: false, subtree: false })
}
