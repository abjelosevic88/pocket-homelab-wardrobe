// A flat drawing of a piece in its colour and pattern: for pieces without
// a photo, and for the things worth buying ("a camel coat like this").
import { useId } from 'react'

function shade(hex: string, amt: number): string {
  const h = hex.replace('#', '')
  if (h.length !== 6) return hex
  const v = [0, 2, 4].map(i => parseInt(h.slice(i, i + 2), 16))
  const out = v.map(c => Math.max(0, Math.min(255, Math.round(amt < 0 ? c * (1 + amt) : c + (255 - c) * amt))))
  return '#' + out.map(c => c.toString(16).padStart(2, '0')).join('')
}

function light(hex: string): boolean {
  const h = hex.replace('#', '')
  const r = parseInt(h.slice(0, 2), 16), g = parseInt(h.slice(2, 4), 16), b = parseInt(h.slice(4, 6), 16)
  return (r * 299 + g * 587 + b * 114) / 1000 > 170
}

const LONG = 'M35 15 L21 21 L9 72 L20 75 L28 44 L28 88 L72 88 L72 44 L80 75 L91 72 L79 21 L65 15 Q50 23 35 15Z'
const SHORT = 'M35 17 L21 23 L11 40 L23 46 L28 39 L28 86 L72 86 L72 39 L77 46 L89 40 L79 23 L65 17 Q50 27 35 17Z'
const OUTER = 'M34 12 L19 18 L7 78 L19 81 L26 46 L26 92 L74 92 L74 46 L81 81 L93 78 L81 18 L66 12 L50 20Z'
const COAT = 'M34 8 L19 14 L8 80 L19 83 L26 46 L24 97 L76 97 L74 46 L81 83 L92 80 L81 14 L66 8 L50 16Z'

/** The outline (and the details drawn on top) of each shape. */
function shapeOf(shape: string, line: string): { body: string; extra?: React.ReactNode; detail?: React.ReactNode } {
  const L = (d: string, w = 1.6) => <path d={d} fill="none" stroke={line} strokeWidth={w} strokeLinecap="round" strokeLinejoin="round" />
  const dots = (pts: [number, number][]) => pts.map(([x, y], i) => <circle key={i} cx={x} cy={y} r={1.4} fill={line} />)
  switch (shape) {
    case 'tank':
      return { body: 'M38 12 L40 28 Q33 38 31 52 L31 88 L69 88 L69 52 Q67 38 60 28 L62 12 L56 12 Q50 24 44 12Z' }
    case 'polo':
      return { body: SHORT, detail: <>{L('M42 21 L50 32 L58 21')}{L('M50 32 L50 44')}{dots([[50, 37], [50, 42]])}</> }
    case 'shirt':
      return { body: LONG, detail: <>{L('M38 17 L46 28 L50 22 L54 28 L62 17')}{L('M50 24 L50 88')}{dots([[52.5, 36], [52.5, 50], [52.5, 64], [52.5, 78]])}</> }
    case 'blouse':
      return { body: LONG, detail: <>{L('M40 17 Q50 32 60 17')}{L('M28 52 Q50 58 72 52', 1.2)}</> }
    case 'longsleeve':
      return { body: LONG, detail: L('M40 16 Q50 24 60 16') }
    case 'sweater':
      return { body: LONG, detail: <>{L('M39 16 Q50 26 61 16')}{L('M28 82 L72 82', 1.2)}{L('M11 68 L21 71', 1.2)}{L('M89 68 L79 71', 1.2)}</> }
    case 'turtleneck':
      return { body: LONG, extra: <path d="M40 6 L60 6 L61 17 Q50 22 39 17Z" />, detail: <>{L('M41 10 L59 10', 1)}{L('M28 82 L72 82', 1.2)}</> }
    case 'cardigan':
      return { body: LONG, detail: <>{L('M39 16 L50 50 L61 16')}{L('M50 50 L50 88')}{dots([[52.5, 58], [52.5, 68], [52.5, 78]])}{L('M28 82 L72 82', 1.2)}</> }
    case 'hoodie':
      return { body: LONG, extra: <path d="M36 16 Q34 2 50 2 Q66 2 64 16 Q50 26 36 16Z" />, detail: <>{L('M44 24 L44 40', 1.2)}{L('M56 24 L56 40', 1.2)}{L('M36 62 L64 62 L60 76 L40 76Z', 1.2)}</> }
    case 'trousers':
      return { body: 'M29 8 L71 8 L75 94 L56 94 L50 34 L44 94 L25 94Z', detail: <>{L('M29 15 L71 15', 1.2)}{L('M50 15 L50 30', 1.2)}</> }
    case 'joggers':
      return { body: 'M29 8 L71 8 L72 86 Q66 92 56 88 L50 34 L44 88 Q34 92 28 86Z', detail: <>{L('M29 16 L71 16', 1.4)}{L('M30 84 L43 84', 1.2)}{L('M57 84 L70 84', 1.2)}</> }
    case 'shorts':
      return { body: 'M29 22 L71 22 L77 66 L55 68 L50 44 L45 68 L23 66Z', detail: L('M29 29 L71 29', 1.2) }
    case 'skirt':
      return { body: 'M34 18 L66 18 L82 84 L18 84Z', detail: <>{L('M34 25 L66 25', 1.2)}{L('M44 25 L38 84', 1)}{L('M56 25 L62 84', 1)}</> }
    case 'dress':
      return { body: 'M40 8 L44 20 Q39 30 38 40 L21 92 L79 92 L62 40 Q61 30 56 20 L60 8 L54 8 Q50 16 46 8Z', detail: L('M38 40 Q50 44 62 40', 1.2) }
    case 'gown':
      return { body: 'M41 4 L44 18 Q39 30 39 44 L14 97 L86 97 L61 44 Q61 30 56 18 L59 4 L54 4 Q50 12 46 4Z', detail: L('M39 44 Q50 48 61 44', 1.2) }
    case 'jumpsuit':
      return { body: 'M38 6 L40 20 Q34 30 32 44 L28 94 L47 94 L50 56 L53 94 L72 94 L68 44 Q66 30 60 20 L62 6 L56 6 Q50 14 44 6Z', detail: L('M32 46 L68 46', 1.2) }
    case 'blazer':
      return { body: OUTER, detail: <>{L('M34 12 L44 50 L50 58 L56 50 L66 12')}{L('M50 20 L44 50')}{L('M50 20 L56 50')}{dots([[50, 66], [50, 76]])}{L('M32 70 L42 70', 1.2)}</> }
    case 'jacket':
      return { body: OUTER, detail: <>{L('M38 13 L50 22 L62 13')}{L('M50 22 L50 92')}{L('M32 60 L42 60', 1.2)}{L('M58 60 L68 60', 1.2)}</> }
    case 'coat':
      return { body: COAT, detail: <>{L('M34 8 L45 46 L50 52 L55 46 L66 8')}{dots([[45, 60], [55, 60], [45, 74], [55, 74]])}{L('M32 70 L40 70', 1.2)}</> }
    case 'puffer':
      return { body: OUTER, detail: <>{L('M50 20 L50 92')}{L('M26 34 L74 34', 1.2)}{L('M26 50 L74 50', 1.2)}{L('M26 66 L74 66', 1.2)}{L('M26 80 L74 80', 1.2)}{L('M16 46 L23 48', 1.2)}{L('M84 46 L77 48', 1.2)}</> }
    case 'parka':
      return { body: COAT, extra: <path d="M36 12 Q34 -2 50 -1 Q66 -2 64 12 Q50 22 36 12Z" />, detail: <>{L('M50 18 L50 97')}{L('M30 58 L42 58 L42 68 L30 68Z', 1.2)}{L('M58 58 L70 58 L70 68 L58 68Z', 1.2)}</> }
    case 'sneaker':
      return { body: 'M8 60 Q10 46 24 44 L38 42 Q50 52 66 52 L84 54 Q93 56 93 66 L93 70 L8 70Z', extra: <path d="M8 70 L93 70 L93 76 Q93 79 89 79 L11 79 Q8 79 8 75Z" fill="#f2f2ef" />, detail: <>{L('M30 50 L34 46', 1.2)}{L('M36 53 L40 48', 1.2)}{L('M42 55 L46 50', 1.2)}</> }
    case 'loafer':
      return { body: 'M8 64 Q10 52 26 52 L58 54 Q80 54 92 64 L92 72 L8 72Z', extra: <path d="M8 72 L92 72 L92 77 L8 77Z" />, detail: L('M34 56 Q46 62 58 56', 1.4) }
    case 'oxford':
      return { body: 'M8 64 Q10 50 26 50 L56 52 Q80 52 92 64 L92 72 L8 72Z', extra: <path d="M8 72 L92 72 L92 76 L8 76Z" />, detail: <>{L('M36 54 L48 60', 1.2)}{L('M48 54 L36 60', 1.2)}{L('M58 54 Q70 56 76 62', 1.2)}</> }
    case 'flat':
      return { body: 'M10 64 Q14 56 28 56 L60 58 Q80 58 90 64 L90 70 L10 70Z', detail: L('M40 58 Q52 62 62 58', 1.2) }
    case 'boot':
      return { body: 'M28 14 L52 14 L54 52 Q72 52 88 60 L90 72 L26 72Z', extra: <path d="M26 72 L90 72 L90 78 L26 78Z" />, detail: <>{L('M30 22 L50 22', 1.2)}{L('M40 14 L40 52', 1)}</> }
    case 'heel':
      return { body: 'M12 66 Q28 48 50 54 L78 50 Q88 50 88 57 L80 59 L78 78 L73 78 L71 62 L30 68 Q18 71 12 66Z' }
    case 'sandal':
      return { body: 'M8 68 Q10 62 22 62 L80 62 Q92 62 92 68 L92 72 L8 72Z', detail: <>{L('M30 62 Q38 46 50 62', 2.4)}{L('M56 62 Q64 50 72 62', 2.4)}</> }
    case 'bag':
      return { body: 'M20 38 L80 38 L86 88 L14 88Z', extra: <path d="M34 38 Q34 14 50 14 Q66 14 66 38 L60 38 Q60 20 50 20 Q40 20 40 38Z" />, detail: L('M20 50 L80 50', 1.2) }
    case 'backpack':
      return { body: 'M26 22 Q26 12 36 12 L64 12 Q74 12 74 22 L76 90 L24 90Z', extra: <path d="M42 12 Q42 4 50 4 Q58 4 58 12Z" fill="none" stroke={line} strokeWidth={2} />, detail: <>{L('M26 38 Q50 46 74 38')}{L('M34 60 L66 60 L66 82 L34 82Z', 1.2)}</> }
    case 'tote':
      return { body: 'M18 36 L82 36 L78 90 L22 90Z', extra: <><path d="M32 36 Q32 12 42 12 Q48 12 48 36" fill="none" stroke={line} strokeWidth={2.4} /><path d="M52 36 Q52 12 58 12 Q68 12 68 36" fill="none" stroke={line} strokeWidth={2.4} /></> }
    case 'belt':
      return { body: 'M4 44 L96 44 L96 56 L4 56Z', extra: <rect x={60} y={40} width={14} height={20} rx={2} fill="none" stroke="#c9a64a" strokeWidth={2.4} />, detail: <>{dots([[30, 50], [38, 50], [46, 50]])}</> }
    case 'scarf':
      return { body: 'M30 10 L70 10 L70 28 L60 28 L66 92 L50 92 L46 30 L42 60 L28 60 L30 28Z', detail: <>{L('M50 88 L50 96', 1.2)}{L('M58 88 L58 96', 1.2)}</> }
    case 'hat':
      return { body: 'M34 56 Q34 24 50 24 Q66 24 66 56Z', extra: <ellipse cx={50} cy={58} rx={42} ry={10} />, detail: L('M34 50 L66 50', 2.4) }
    case 'cap':
      return { body: 'M24 60 Q24 30 50 30 Q76 30 76 60Z', extra: <path d="M64 58 Q84 56 94 64 L70 66Z" />, detail: <>{L('M50 30 L50 60', 1)}<circle cx={50} cy={30} r={2.4} fill={line} /></> }
    case 'beanie':
      return { body: 'M24 70 Q24 30 50 28 Q76 30 76 70Z', extra: <><circle cx={50} cy={24} r={8} /><path d="M22 66 L78 66 L78 80 L22 80Z" /></>, detail: <>{L('M30 66 L30 80', 1)}{L('M40 66 L40 80', 1)}{L('M50 66 L50 80', 1)}{L('M60 66 L60 80', 1)}{L('M70 66 L70 80', 1)}</> }
    case 'tie':
      return { body: 'M44 12 L56 12 L54 22 L62 76 L50 90 L38 76 L46 22Z', detail: L('M46 22 L54 22', 1.2) }
    case 'watch':
      return { body: 'M42 6 L58 6 L58 94 L42 94Z', extra: <circle cx={50} cy={50} r={17} fill="#e9eaed" stroke={line} strokeWidth={3} />, detail: <>{L('M50 50 L50 40', 1.6)}{L('M50 50 L57 54', 1.6)}</> }
    case 'sunglasses':
      return { body: 'M8 42 L44 42 Q44 66 26 66 Q8 66 8 42Z M56 42 L92 42 Q92 66 74 66 Q56 66 56 42Z', detail: L('M44 46 Q50 40 56 46', 2) }
    case 'jewellery':
      return { body: 'M50 76 L58 88 L50 96 L42 88Z', extra: <path d="M20 14 Q50 84 80 14" fill="none" stroke={line} strokeWidth={2} /> }
    case 'gloves':
      return { body: 'M30 92 L30 50 L24 34 Q22 28 28 28 L34 40 L34 18 Q34 12 39 12 Q43 12 43 18 L43 36 L45 12 Q46 6 50 6 Q55 6 54 12 L52 36 L56 16 Q58 10 62 12 Q66 14 64 20 L60 40 L66 28 Q70 24 72 30 L64 56 L64 92Z', detail: L('M30 82 L64 82', 1.4) }
    case 'swim':
      return { body: 'M24 30 L76 30 L74 50 Q62 54 56 70 L44 70 Q38 54 26 50Z', detail: L('M24 36 L76 36', 1.2) }
    case 'tshirt':
    default:
      return { body: SHORT, detail: L('M40 18 Q50 27 60 18') }
  }
}

/** A piece drawn flat; size is its box in pixels. */
export function Garment({ shape, color = '#9a9ca0', pattern = 'plain', size = 96, title, second }: { shape: string; color?: string; pattern?: string; size?: number; title?: string; second?: string }) {
  const id = useId().replace(/:/g, '')
  const line = light(color) ? shade(color, -0.32) : shade(color, 0.35)
  const ink = second || (light(color) ? shade(color, -0.4) : shade(color, 0.45))
  const s = shapeOf(shape, line)
  const pat = pattern && pattern !== 'plain'
  return (
    <svg className="garment" viewBox="-2 -4 104 104" width={size} height={size} role="img" aria-label={title || shape}>
      {title && <title>{title}</title>}
      <defs>
        {pattern === 'stripes' && <pattern id={`p${id}`} width="8" height="8" patternUnits="userSpaceOnUse"><rect width="8" height="3" fill={ink} opacity=".55" /></pattern>}
        {pattern === 'checks' && <pattern id={`p${id}`} width="12" height="12" patternUnits="userSpaceOnUse"><rect width="12" height="4" fill={ink} opacity=".35" /><rect width="4" height="12" fill={ink} opacity=".35" /></pattern>}
        {pattern === 'dots' && <pattern id={`p${id}`} width="9" height="9" patternUnits="userSpaceOnUse"><circle cx="4.5" cy="4.5" r="1.7" fill={ink} opacity=".7" /></pattern>}
        {pattern === 'floral' && <pattern id={`p${id}`} width="16" height="16" patternUnits="userSpaceOnUse"><g fill={ink} opacity=".6"><circle cx="5" cy="5" r="2" /><circle cx="8" cy="4" r="1.5" /><circle cx="6" cy="8" r="1.5" /><circle cx="13" cy="13" r="1.6" /></g></pattern>}
        {(pattern === 'print' || pattern === 'other') && <pattern id={`p${id}`} width="10" height="10" patternUnits="userSpaceOnUse" patternTransform="rotate(35)"><rect width="10" height="2.5" fill={ink} opacity=".4" /></pattern>}
        <clipPath id={`c${id}`}><path d={s.body} /></clipPath>
      </defs>
      <g fill={color} stroke={line} strokeWidth={1.6} strokeLinejoin="round">
        {s.extra}
        <path d={s.body} />
      </g>
      {pat && <rect x="-2" y="-4" width="104" height="104" fill={`url(#p${id})`} clipPath={`url(#c${id})`} />}
      <path d={s.body} fill="url(#none)" style={{ fill: 'none' }} stroke={line} strokeWidth={1.6} strokeLinejoin="round" />
      {s.detail}
    </svg>
  )
}
