import { cva, cx } from 'class-variance-authority'
import { ExclamationCircleIcon } from '@heroicons/react/20/solid'
import { forwardRef } from 'react'
import { FieldError, UseFormRegisterReturn } from 'react-hook-form'

const inputContainerStyles = cva(
  'flex relative rounded-md border focus-within:ring-1 w-full font-normal overflow-hidden',
  {
    variants: {
      size: {
        sm: 'text-sm h-6',
        md: 'text-base h-8',
        lg: 'text-lg h-10'
      },
      error: {
        true: 'border-red-300 text-red-900 placeholder:text-red-300 focus-within:ring-red-500',
        false:
          'border-gray-300  focus-within:ring-primary-500 focus-within:border-primary-500'
      },
      readOnly: {
        true: 'bg-gray-50 text-gray-400',
        false: 'text-text-foreground'
      }
    },
    defaultVariants: {
      size: 'md',
      error: false,
      readOnly: false
    },
    compoundVariants: []
  }
)

const inputStyles = cva(
  [
    'block',
    'w-full',
    'placeholder:text-ilabel placeholder:font-normal',
    'border-none focus:ring-0',
    'focus:outline-none'
  ],
  {
    variants: {
      size: {
        sm: 'sm:text-xs placeholder:text-xs placeholder:font-normal pl-2 pr-6',
        md: 'sm:text-ilabel placeholder:text-ilabel placeholder:font-normal pl-2 pr-10',
        lg: 'sm:text-lg placeholder:text-lg placeholder:font-normal pl-3 pr-10'
      },
      error: {
        true: 'border-red-300',
        false: 'border-gray-300 '
      }
    },
    defaultVariants: {
      size: 'md',
      error: false
    }
  }
)

const iconStyles = cva('text-red-500', {
  variants: {
    size: {
      sm: 'h-4 w-4',
      md: 'h-5 w-5',
      lg: 'h-6 w-6'
    }
  },
  defaultVariants: {
    size: 'md'
  }
})

type InputProps = UseFormRegisterReturn & {
  error?: FieldError
  errorClassName?: string
  size?: 'sm' | 'md' | 'lg'
  className?: string
  value?: string
  placeholder?: string
} & React.InputHTMLAttributes<HTMLInputElement>

export const Input = forwardRef<HTMLInputElement, InputProps>(
  function Input(props, inputRef) {
    const { className, error, errorClassName, size, ...rest } = props

    const containerClassName = inputContainerStyles({
      size,
      error: !!error,
      readOnly: rest.disabled
    })
    const inputClassName = cx(inputStyles({ size, error: !!error }), className)

    return (
      <div>
        <div className={containerClassName}>
          <input {...rest} ref={inputRef} className={inputClassName} />
          {error && (
            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
              <ExclamationCircleIcon className={iconStyles({ size })} />
            </div>
          )}
        </div>
        {error && (
          <p className="mt-2 text-xs text-red-600">
            {typeof error == 'string' ? error : error.message}
          </p>
        )}
      </div>
    )
  }
)
