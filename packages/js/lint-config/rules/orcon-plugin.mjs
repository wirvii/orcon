// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Small local ESLint plugin with the rules that no published plugin provides.
// Each rule has its own id so a disable directive silences exactly one rule.

const HEX = /(?<![\w&/#-])#(?:[0-9a-f]{8}|[0-9a-f]{6}|[0-9a-f]{3,4})(?![\w-])/i;
const COLOR_FUNCTION = /\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/i;

function hasColorLiteral(text) {
  return HEX.test(text) || COLOR_FUNCTION.test(text);
}

const noColorLiteral = {
  meta: {
    type: 'problem',
    docs: {
      description: 'Disallow hex, rgb() and hsl() color literals; colors come from tokens.',
    },
    schema: [],
    messages: {
      literal:
        'Color literals are not allowed. Use a token utility or var(--orcon-*) from @orcon/tokens.',
    },
  },
  create(context) {
    return {
      Literal(node) {
        if (typeof node.value === 'string' && hasColorLiteral(node.value)) {
          context.report({ node, messageId: 'literal' });
        }
      },
      TemplateElement(node) {
        if (hasColorLiteral(node.value.raw)) {
          context.report({ node, messageId: 'literal' });
        }
      },
    };
  },
};

const noInlineStyle = {
  meta: {
    type: 'problem',
    docs: {
      description:
        'Disallow the JSX style attribute. Canvas positioning may disable this rule with a stated reason.',
    },
    schema: [],
    messages: {
      inline:
        'Inline style is not allowed. Only runtime canvas positions may use it, with a disable comment that states the reason.',
    },
  },
  create(context) {
    return {
      JSXAttribute(node) {
        if (node.name.type === 'JSXIdentifier' && node.name.name === 'style') {
          context.report({ node, messageId: 'inline' });
        }
      },
    };
  },
};

export default {
  meta: { name: '@orcon/lint-config/orcon' },
  rules: {
    'no-color-literal': noColorLiteral,
    'no-inline-style': noInlineStyle,
  },
};
