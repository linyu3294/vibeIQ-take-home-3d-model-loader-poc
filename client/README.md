## Prerequisites

- **Node.js v18 or higher is required.**
  - You can check your version with `node -v`.
  - If you use [nvm](https://github.com/nvm-sh/nvm), run `nvm use 18` or `nvm install 18`.

## Client Setup & Usage

### 1. Install dependencies

Navigate to the `client` directory and install dependencies:

```bash
cd client
npm install
```

### 2. Configure environment variables

Create a `.env` file in the `client` directory with the following content:

```
VITE_API_GATEWAY_URL=your_api_gateway_url
VITE_API_KEY=your_api_key
```

Replace `your_api_gateway_url` and `your_api_key` with your actual API Gateway endpoint and API key.

### 3. Run the development server

```bash
npm run dev
```

This will start the Vite development server. By default, the app will be available at [http://localhost:5173](http://localhost:5173).

### 4. Using the App

- The app will fetch a presigned URL for a 3D model from your API and load it using Three.js.
- Make sure your S3 bucket CORS policy is set up as described in the server README.