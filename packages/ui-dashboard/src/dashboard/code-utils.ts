import isEmpty from 'lodash/isEmpty'

/** Escape single quotes so a JSON body survives inside a single-quoted shell string. */
export function escapeBody(body: unknown) {
  if (typeof body !== 'string') return body
  return body.replace(/'/g, `'\\''`)
}

export function generateNodeCode(
  url: string,
  data: unknown,
  headers?: Record<string, unknown>,
  apiKey = '<API_KEY>',
  method = 'POST',
  apiKeyPosition: 'header' | 'param' = 'header'
) {
  if (apiKeyPosition === 'header') {
    headers = headers
      ? { ...headers, 'api-key': apiKey }
      : { 'api-key': apiKey }
  } else {
    url = url.includes('?')
      ? `${url}&api-key=${apiKey}`
      : `${url}?api-key=${apiKey}`
  }
  return `const fetch = (...args) => import('node-fetch').then(({default: fetch}) => fetch(...args));

const url = '${url}';
const data = ${JSON.stringify(data, null, 2)};
const method = '${method}';
${headers ? `const headers = ${JSON.stringify(headers, null, 2)};` : ''}

fetch(url, {
  method,
  headers: {
    'Content-Type': 'application/json',${headers ? '\n    ...headers,' : ''}
  },
  body: method === 'GET' ? undefined : JSON.stringify(data),
})
  .then(response => {
    if (!response.ok) {
      throw new Error('Network response was not ok');
    }
    return response.json();
  })
  .then(data => {
    // Do something with the data
    console.log(data);
  })
  .catch(error => {
    // Handle the error
    console.error('Error:', error);
  });

`
}

export function generateCurlCode(
  url: string,
  data: unknown,
  headers?: Record<string, unknown>,
  apiKey = '<API_KEY>',
  method = 'POST',
  apiKeyPosition: 'header' | 'param' = 'header'
) {
  if (apiKeyPosition === 'header') {
    headers = headers
      ? { ...headers, 'api-key': apiKey }
      : { 'api-key': apiKey }
  } else {
    url = url.includes('?')
      ? `${url}&api-key=${apiKey}`
      : `${url}?api-key=${apiKey}`
  }

  let curlCommand = `curl -L -X ${method} '${url}' \\
     -H 'Content-Type: application/json'`

  if (headers && !isEmpty(headers)) {
    curlCommand +=
      ' \\\n' +
      Object.entries(headers)
        .map(([key, value]) => `     -H '${key}: ${value}' \\`)
        .join('\n')
  } else {
    curlCommand += ` \\`
  }

  if (data) {
    curlCommand += `\n     --data-raw '${escapeBody(JSON.stringify(data, null, 2))}'`
  } else {
    // remove last backslash
    curlCommand = curlCommand.slice(0, -1)
  }

  return curlCommand
}
