export default {
  testEnvironment: 'jsdom',
  transform: {
    "\\.[jt]s?$": "babel-jest",
  },
  setupFilesAfterEnv: ['<rootDir>/jest.setup.js'],
};