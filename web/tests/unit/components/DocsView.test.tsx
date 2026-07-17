import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DocsView } from '../../../src/components/DocsView';
import type { DocIndex } from 'go-ui';

// A minimal DocIndex the stubbed fetch returns for DocsApp's doc.json request.
const DOC_INDEX: DocIndex = {
  module: 'github.com/malcolmston/sharp',
  packages: [
    {
      importPath: 'github.com/malcolmston/sharp',
      name: 'sharp',
      synopsis: 'Package sharp is a fluent, standard-library-only image-processing library for Go.',
      doc: 'Package sharp is a fluent, standard-library-only image-processing library for Go.',
      consts: [],
      vars: [],
      types: [
        {
          name: 'Pipeline',
          signature: 'type Pipeline struct{}',
          doc: 'Pipeline is a fluent, chainable image-processing pipeline.',
          consts: [],
          vars: [],
          funcs: [],
          methods: [],
        },
      ],
      funcs: [{ name: 'FromFile', signature: 'func FromFile(path string) *Pipeline', doc: 'FromFile reads and decodes an image file from disk.' }],
    },
  ],
};

describe('DocsView', () => {
  beforeEach(() => {
    // DocsApp fetches doc.json; return the small index.
    global.fetch = vi.fn((input: RequestInfo | URL) => {
      if (String(input).includes('doc.json')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(DOC_INDEX) } as Response);
      }
      return new Promise<Response>(() => {});
    }) as unknown as typeof fetch;
  });

  it('renders the inline React API reference from the fetched doc.json', async () => {
    const { container } = render(<DocsView />);
    expect(container.querySelector('#view-docs')).not.toBeNull();
    expect(
      screen.getByRole('heading', { level: 2, name: /API documentation/ }),
    ).toBeInTheDocument();

    // DocsApp fetches asynchronously, then renders the package view + symbols.
    expect(await screen.findByRole('heading', { name: /package sharp/ })).toBeInTheDocument();
    expect(container.querySelector('#sym-FromFile'), 'func FromFile symbol card').not.toBeNull();
    expect(container.querySelector('#sym-Pipeline'), 'type Pipeline symbol card').not.toBeNull();

    // The secondary link to the raw generated static HTML remains.
    expect(screen.getByRole('link', { name: /Open the raw generated HTML/ })).toHaveAttribute('href', './api/');
  });
});
