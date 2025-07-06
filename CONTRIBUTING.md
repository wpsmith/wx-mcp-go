# Contributing to Swagger Documentation MCP Server

Thank you for your interest in contributing to the Swagger Documentation MCP Server! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites
- Node.js 18+
- npm or yarn
- Git

### Development Setup

1. **Fork the repository**
   ```bash
   git clone https://github.com/your-username/swagger-docs-mcp.git
   cd swagger-docs-mcp
   ```

2. **Install dependencies**
   ```bash
   npm install
   ```

3. **Build the project**
   ```bash
   npm run build
   ```

4. **Run tests**
   ```bash
   npm test
   ```

### Development Scripts

```bash
npm run build      # Compile TypeScript
npm run dev        # Watch mode compilation
npm run test       # Run tests
npm run lint       # ESLint
npm run format     # Prettier
npm run clean      # Clean build artifacts
```

## Project Structure

```
src/
├── server/          # MCP server implementation
│   ├── mcp-server.ts
│   └── tool-registry.ts
├── swagger/         # Swagger processing
│   ├── document-parser.ts
│   ├── tool-generator.ts
│   └── schema-converter.ts
├── http/            # HTTP client and authentication
│   ├── http-client.ts
│   └── auth-handler.ts
├── types/           # TypeScript definitions
├── utils/           # Shared utilities
├── cli.ts           # CLI interface
└── index.ts         # Main entry point
```

## Development Guidelines

### Code Style

- **TypeScript**: Use strict TypeScript with no `any` types
- **ESLint**: Follow the project's ESLint configuration
- **Prettier**: Code is automatically formatted with Prettier
- **Naming**: Use descriptive names for variables, functions, and classes

### Design Patterns

The project follows these design patterns:
- **Registry Pattern**: For tool management
- **Factory Pattern**: For creating tools and clients
- **Strategy Pattern**: For authentication methods
- **Command Pattern**: For tool execution
- **Pipeline Pattern**: For request/response processing

### Error Handling

- Use the Result pattern instead of throwing exceptions where possible
- Provide meaningful error messages
- Log errors appropriately with context
- Handle edge cases gracefully

### Testing

- Write unit tests for all new functionality
- Use integration tests for complex workflows
- Mock external dependencies
- Aim for 90%+ test coverage on critical paths

### Documentation

- Document all public APIs with JSDoc comments
- Update README.md for user-facing changes
- Add inline comments for complex logic
- Keep documentation up to date with code changes

## Contributing Process

### 1. Create an Issue

Before starting work, create an issue to discuss:
- Bug reports with reproduction steps
- Feature requests with use cases
- Questions about implementation

### 2. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/bug-description
```

### 3. Make Your Changes

- Follow the coding standards
- Write tests for new functionality
- Update documentation as needed
- Keep commits focused and atomic

### 4. Test Your Changes

```bash
# Run all tests
npm test

# Run linting
npm run lint

# Build the project
npm run build

# Test with real swagger documents
npm run test:integration
```

### 5. Submit a Pull Request

- Write a clear PR description
- Reference related issues
- Include screenshots for UI changes
- Ensure all CI checks pass

## Types of Contributions

### Bug Fixes
- Fix issues reported in GitHub issues
- Add regression tests
- Update documentation if needed

### New Features
- Implement new MCP server capabilities
- Add support for new swagger features
- Enhance authentication methods
- Improve error handling

### Documentation
- Improve README and guides
- Add code examples
- Fix typos and clarify instructions
- Add troubleshooting information

### Performance
- Optimize swagger parsing
- Improve tool generation speed
- Reduce memory usage
- Add performance benchmarks

### Testing
- Add missing test coverage
- Improve test reliability
- Add integration tests
- Create test utilities

## Code Review Process

### What We Look For

- **Correctness**: Does the code work as intended?
- **Testing**: Are there adequate tests?
- **Documentation**: Is the code well-documented?
- **Performance**: Are there any performance implications?
- **Security**: Are there any security concerns?
- **Maintainability**: Is the code easy to understand and modify?

### Review Timeline

- Initial review within 2-3 business days
- Follow-up reviews within 1-2 business days
- Merge after approval and passing CI

## Release Process

### Versioning

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Checklist

1. Update version in `package.json`
2. Update CHANGELOG.md
3. Run full test suite
4. Create release PR
5. Tag release after merge
6. Publish to npm

## Getting Help

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and general discussion
- **Pull Request Comments**: Code-specific discussions

### Resources

- [MCP Specification](https://modelcontextprotocol.io/docs)
- [OpenAPI Specification](https://swagger.io/specification/)
- [TypeScript Documentation](https://www.typescriptlang.org/docs/)

## Recognition

Contributors will be:
- Listed in the project's contributors section
- Mentioned in release notes for significant contributions
- Invited to join the maintainer team for sustained contributions

## Code of Conduct

### Our Standards

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Maintain a professional environment

### Enforcement

Violations of the code of conduct should be reported to the project maintainers. All reports will be handled confidentially.

## License

By contributing to this project, you agree that your contributions will be licensed under the same MIT License that covers the project.

---

Thank you for contributing to the Swagger Documentation MCP Server! Your contributions help make this tool better for everyone.
