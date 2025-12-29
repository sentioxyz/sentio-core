import { FunctionType } from './types'

const FunctionIcon = (args: React.SVGProps<SVGSVGElement>) => (
  <svg
    width="16"
    height="16"
    viewBox="0 0 16 16"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    {...args}
  >
    <g clipPath="url(#clip0_6732_6574)">
      <path
        d="M10 8H10.0067"
        stroke="currentColor"
        strokeWidth="1.33333"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M8 8H8.00667"
        stroke="currentColor"
        strokeWidth="1.33333"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M6 8H6.00667"
        stroke="currentColor"
        strokeWidth="1.33333"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M4 12.6654C3.64638 12.6654 3.30724 12.5249 3.05719 12.2748C2.80714 12.0248 2.66667 11.6857 2.66667 11.332V8.66536L2 7.9987L2.66667 7.33203V4.66536C2.66667 4.31174 2.80714 3.9726 3.05719 3.72256C3.30724 3.47251 3.64638 3.33203 4 3.33203"
        stroke="currentColor"
        strokeWidth="1.33333"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M12 12.6654C12.3536 12.6654 12.6928 12.5249 12.9428 12.2748C13.1929 12.0248 13.3333 11.6857 13.3333 11.332V8.66536L14 7.9987L13.3333 7.33203V4.66536C13.3333 4.31174 13.1929 3.9726 12.9428 3.72256C12.6928 3.47251 12.3536 3.33203 12 3.33203"
        stroke="currentColor"
        strokeWidth="1.33333"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </g>
    <defs>
      <clipPath id="clip0_6732_6574">
        <rect width="16" height="16" fill="white" />
      </clipPath>
    </defs>
  </svg>
)

interface Props {
  active?: boolean
  selected?: boolean
  data: FunctionType
}

export const FunctionOption = ({ data, active, selected }: Props) => {
  const { name, inputs, outputs } = data

  return (
    <div className="flex items-center gap-4">
      <div>
        <FunctionIcon className="text-text-foreground h-4 w-4" />
      </div>
      <div className="text-text-foreground text-xs font-medium">
        <div>
          <span>{name}</span>
          <span className="text-gray">(</span>
          {inputs.map((input, index) => (
            <span
              key={`${name}.input.${index}`}
              className={index !== inputs.length - 1 ? 'mr-1' : ''}
            >
              <span className="text-primary">
                {input.internalType ?? input.type}
              </span>
              <span className="ml-1">{input.name}</span>
              {index < inputs.length - 1 && <span>, </span>}
            </span>
          ))}
          <span className="text-gray">)</span>
        </div>
        <div>
          <span className="mr-1">Return Value:</span>
          {outputs?.length > 0 ? (
            outputs.map((output, index) => (
              <span key={`${name}.input.${index}`}>
                <span className="text-primary">
                  {output.internalType ?? output.type}
                </span>
                {index < outputs.length - 1 && <span>, </span>}
              </span>
            ))
          ) : (
            <span>-</span>
          )}
        </div>
      </div>
    </div>
  )
}
