import { useState } from "react";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import { Providers } from "./app/Providers";
import { createQueryClient } from "./app/queryClient";
import { routes } from "./app/routes";

const router = createBrowserRouter(routes);

export default function App() {
  const [client] = useState(createQueryClient);
  return (
    <Providers client={client}>
      <RouterProvider router={router} />
    </Providers>
  );
}
