// callee_sub_146E8BA20

__int64 __fastcall sub_146E8BA20(__int64 a1)
{
  __int64 result; // rax
  int v3; // ebx
  _BYTE pExceptionObject[40]; // [rsp+20h] [rbp-28h] BYREF

  result = ((__int64 (*)(void))sub_14885EFED)();
  if ( !result )
  {
    v3 = 0;
    while ( 1 )
    {
      result = sub_14885EFED(a1);
      if ( result )
        break;
      MEMORY[0xDC5D2F6](500);
      if ( ++v3 >= 20 )
      {
        if ( dword_14F0FC2F0 > *(_DWORD *)(*((_QWORD *)NtCurrentTeb()->ThreadLocalStoragePointer
                                           + (unsigned int)dword_14F3BEE58)
                                         + 420620LL) )
        {
          sub_148860450(&dword_14F0FC2F0);
          if ( dword_14F0FC2F0 == -1 )
          {
            sub_1412EFC00(&qword_14F0FC2D8);
            sub_14885FFE8(sub_14904C2E0);
            sub_1488603F0(&dword_14F0FC2F0);
          }
        }
        sub_14014AC30(pExceptionObject, &qword_14F0FC2D8);
        throw (std::bad_alloc *)pExceptionObject;
      }
    }
  }
  return result;
}

