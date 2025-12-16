import '../styles.css'
import { CopyButton } from './CopyButton';

export const Default = () => (
  <div className="p-4">
    <CopyButton text="Hello, World!" />
  </div>
);

export const WithCustomText = () => (
  <div className="p-4">
    <CopyButton text="This is a longer text to copy to the clipboard." />
  </div>
);

export const NoHint = () => (
  <div className="p-4">
    <CopyButton text="Copy this text" noHint />
  </div>
);

export const WithCustomChildren = () => (
  <div className="p-4">
    <CopyButton text="Custom button text">
      <button className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600">
        Copy Me
      </button>
    </CopyButton>
  </div>
);

export const FixedHint = () => (
  <div className="p-4">
    <CopyButton text="Fixed hint example" hintFixed />
  </div>
);
