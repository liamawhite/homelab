import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { userClient } from "./client";

const usersQueryKey = ["users"];

export function useUsers() {
  return useQuery({
    queryKey: usersQueryKey,
    queryFn: async () => (await userClient.listUsers({})).users,
  });
}

export function useCreateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (name: string) => {
      const res = await userClient.createUser({ name });
      if (!res.user) {
        throw new Error("CreateUser response did not include a user");
      }
      return res.user;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
  });
}

export function useDeleteUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (id: string) => {
      await userClient.deleteUser({ id });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: usersQueryKey });
    },
  });
}
